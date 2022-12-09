package __latency_testing_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"

	performancev2 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/performanceprofile/v2"
	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/images"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/junit"
	testlog "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/log"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/namespaces"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profiles"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profilesupdate"

	ginkgo_reporters "kubevirt.io/qe-tools/pkg/ginkgo-reporters"
)

// TODO get commonly used variables from one shared file that defines constants
const testExecutablePath = "../../../../../build/_output/bin/latency-e2e.test"

var prePullNamespace = &corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name: "testing-prepull",
	},
}
var profile, initialProfile *performancev2.PerformanceProfile

var _ = BeforeSuite(func() {
	Expect(isTestExecutableFound()).To(BeTrue())
	Expect(testclient.ClientsEnabled).To(BeTrue())

	// update PP isolated CPUs. the new cpu set for isolated should have an even number of CPUs to avoid failing the pod on SMTAlignment error,
	// and should be greater than what is requested by the test cases in the suite so the test runs properly
	var err error
	profile, err = profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
	Expect(err).ToNot(HaveOccurred())

	By("Backing up the profile")
	initialProfile = profile.DeepCopy()

	By(fmt.Sprintf("verify if the isolated cpus value under the performance profile %q is appropriate for this test suite", profile.Name))
	workerNodes, err := nodes.GetByLabels(testutils.NodeSelectorLabels)
	Expect(err).ToNot(HaveOccurred())

	initialIsolated := profile.Spec.CPU.Isolated
	initialReserved := profile.Spec.CPU.Reserved
	var numaCoreSiblings map[int]map[int][]int
	var reserved, isolated []string

	for _, node := range workerNodes {
		numaCoreSiblings, err = nodes.GetCoreSiblings(&node)
	}
	for reservedCores := 0; reservedCores < 2; reservedCores++ {
		// Get the cpu siblings from the selected core and delete the siblings
		// from the map. Selected siblings of cores are saved in reservedCpus
		cpusiblings := nodes.GetCpuSiblings(numaCoreSiblings, reservedCores)
		reserved = append(reserved, cpusiblings...)
	}
	reservedCpus := strings.Join(reserved, ",")

	for node := range numaCoreSiblings {
		for core := range numaCoreSiblings[node] {
			cpusiblings := nodes.GetCpuSiblings(numaCoreSiblings, core)
			isolated = append(isolated, cpusiblings...)
		}
	}
	isolatedCpus := strings.Join(isolated, ",")
	//updated both sets to ensure there is no overlap
	latencyIsolatedSet := performancev2.CPUSet(isolatedCpus)
	latencyReservedSet := performancev2.CPUSet(reservedCpus)
	testlog.Infof("current isolated cpus: %s, desired is %s", string(*initialIsolated), latencyIsolatedSet)
	totalCpus := cpuset.MustParse(string(latencyIsolatedSet)).Size() + cpuset.MustParse(string(latencyReservedSet)).Size()
	nodesWithSufficientCpu := nodes.GetByCpuCapacity(workerNodes, totalCpus)
	//before applying the changes verify that there are compute nodes with sufficient cpus
	Expect(len(nodesWithSufficientCpu)).NotTo(Equal(0), "found 0 nodes with sufficient cpus %d for the performance profile configuration.", totalCpus)

	if *initialIsolated != latencyIsolatedSet || *initialReserved != latencyReservedSet {
		By("Update the isolated and reserved cpus sets of the profile")
		err = profilesupdate.UpdateIsolatedReservedCpus(profile, latencyIsolatedSet, latencyReservedSet)
		Expect(err).ToNot(HaveOccurred(), "could not update the profile with the desired CPUs sets")
	}

	if err := createNamespace(); err != nil {
		testlog.Errorf("cannot create the namespace: %v", err)
	}

	ds, err := images.PrePull(testclient.Client, images.Test(), prePullNamespace.Name, "cnf-tests")
	if err != nil {
		data, _ := json.Marshal(ds) // we can safely skip errors
		testlog.Infof("DaemonSet %s/%s image=%q status:\n%s", ds.Namespace, ds.Name, images.Test(), string(data))
		testlog.Errorf("cannot prepull image %q: %v", images.Test(), err)
	}
})

var _ = AfterSuite(func() {
	prePullNamespaceName := prePullNamespace.Name
	err := testclient.Client.Delete(context.TODO(), prePullNamespace)
	if err != nil {
		testlog.Errorf("namespace %q could not be deleted err=%v", prePullNamespace.Name, err)
	}
	namespaces.WaitForDeletion(prePullNamespaceName, 5*time.Minute)
	currentProfile, err := profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
	Expect(err).ToNot(HaveOccurred())
	if reflect.DeepEqual(currentProfile.Spec, initialProfile.Spec) != true {
		By("Restore initial performance profile")
		err = profilesupdate.ApplyProfile(initialProfile)
		if err != nil {
			testlog.Errorf("could not restore the initial profile: %v", err)
		}
	}
})

func Test5LatencyTesting(t *testing.T) {
	RegisterFailHandler(Fail)

	rr := []Reporter{}
	if ginkgo_reporters.Polarion.Run {
		rr = append(rr, &ginkgo_reporters.Polarion)
	}
	rr = append(rr, junit.NewJUnitReporter("latency_testing"))
	RunSpecsWithDefaultAndCustomReporters(t, "Performance Addon Operator latency tools testing", rr)
}

func createNamespace() error {
	err := testclient.Client.Create(context.TODO(), prePullNamespace)
	if errors.IsAlreadyExists(err) {
		testlog.Warningf("%q namespace already exists, that is unexpected", prePullNamespace.Name)
		return nil
	}
	testlog.Infof("created namespace %q err=%v", prePullNamespace.Name, err)
	return err
}

func isTestExecutableFound() bool {
	if _, err := os.Stat(testExecutablePath); os.IsNotExist(err) {
		return false
	}
	return true
}
