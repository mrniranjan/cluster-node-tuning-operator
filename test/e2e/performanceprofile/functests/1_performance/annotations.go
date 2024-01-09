package __performance

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"

	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/utils/schedstat"
	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/cgroups"
	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	testlog "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/log"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mcps"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/pods"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profiles"
	machineconfigv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
)

const (
	cgroupRoot = "/rootfs/sys/fs/cgroup"
)

var _ = Describe("[performance] Test crio annotaions on OCI runtimes", Ordered, Label("crio"), func() {
	var performanceMCP string
	var smtLevel int
	ctx := context.Background()
	testutils.CustomBeforeAll(func() {
		var workerRTNodes []corev1.Node
		workerRTNodes, err := nodes.GetByLabels(testutils.NodeSelectorLabels)
		Expect(err).ToNot(HaveOccurred())

		workerRTNodes, err = nodes.MatchingOptionalSelector(workerRTNodes)
		Expect(err).ToNot(HaveOccurred())

		workerRTNode = &workerRTNodes[0]

		profile, err = profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
		Expect(err).ToNot(HaveOccurred())

		performanceMCP, err = mcps.GetByProfile(profile)
		Expect(err).ToNot(HaveOccurred())

		onlineCPUSet, err := nodes.GetOnlineCPUsSet(workerRTNode)
		Expect(err).ToNot(HaveOccurred())

		for _, mcpName := range []string{testutils.RoleWorker, performanceMCP} {
			mcps.WaitForCondition(mcpName, machineconfigv1.MachineConfigPoolUpdated, corev1.ConditionTrue)
		}

		cpuID := onlineCPUSet.UnsortedList()[0]
		smtLevel = nodes.GetSMTLevel(cpuID, workerRTNode)

	})

	Context("Crio Annotations", func() {
		var testpod *corev1.Pod
		var allTestpods map[types.UID]*corev1.Pod
		annotations := map[string]string{
			"cpu-load-balancing.crio.io": "disable",
			"cpu-quota.crio.io":          "disable",
		}
		BeforeAll(func() {
			var err error
			allTestpods = make(map[types.UID]*corev1.Pod)

			testpod = getTestPodWithAnnotations(annotations, smtLevel)
			// workaround for https://github.com/kubernetes/kubernetes/issues/107074
			// until https://github.com/kubernetes/kubernetes/pull/120661 lands
			unblockerPod := pods.GetTestPod() // any non-GU pod will suffice ...
			unblockerPod.Namespace = testutils.NamespaceTesting
			unblockerPod.Spec.NodeSelector = map[string]string{testutils.LabelHostname: workerRTNode.Name}
			err = testclient.Client.Create(context.TODO(), unblockerPod)
			Expect(err).ToNot(HaveOccurred())
			allTestpods[unblockerPod.UID] = unblockerPod
			time.Sleep(30 * time.Second) // let cpumanager reconcile loop catch up

			// It's possible that when this test runs the value of
			// defaultCpuNotInSchedulingDomains is empty if no gu pods are running
			defaultCpuNotInSchedulingDomains, err := getCPUswithLoadBalanceDisabled(workerRTNode)
			Expect(err).ToNot(HaveOccurred(), "Unable to fetch scheduling domains")

			if len(defaultCpuNotInSchedulingDomains) > 0 {
				pods, err := pods.GetPodsOnNode(context.TODO(), workerRTNode.Name)
				if err != nil {
					testlog.Warningf("cannot list pods on %q: %v", workerRTNode.Name, err)
				} else {
					testlog.Infof("pods on %q BEGIN", workerRTNode.Name)
					for _, pod := range pods {
						testlog.Infof("- %s/%s %s", pod.Namespace, pod.Name, pod.UID)
					}
					testlog.Infof("pods on %q END", workerRTNode.Name)
				}
				Expect(defaultCpuNotInSchedulingDomains).To(BeEmpty(), "the test expects all CPUs within a scheduling domain when starting")
			}

			By("Starting the pod")
			testpod.Spec.NodeSelector = testutils.NodeSelectorLabels
			testpod.Spec.Containers[0].Image = "quay.io/mniranja/busycpus"
			err = testclient.Client.Create(context.TODO(), testpod)
			Expect(err).ToNot(HaveOccurred())
			testpod, err = pods.WaitForCondition(client.ObjectKeyFromObject(testpod), corev1.PodReady, corev1.ConditionTrue, 10*time.Minute)
			logEventsForPod(testpod)
			Expect(err).ToNot(HaveOccurred(), "failed to create guaranteed pod %v", testpod)
			allTestpods[testpod.UID] = testpod
		})

		AfterAll(func() {
			for podUID, testpod := range allTestpods {
				testlog.Infof("deleting test pod %s/%s UID=%q", testpod.Namespace, testpod.Name, podUID)
				err := testclient.Client.Get(context.TODO(), client.ObjectKeyFromObject(testpod), testpod)
				Expect(err).ToNot(HaveOccurred())
				//testpodUID := testpod.UID
				err = testclient.Client.Delete(context.TODO(), testpod)
				Expect(err).ToNot(HaveOccurred())

				err = pods.WaitForDeletion(testpod, pods.DefaultDeletionTimeout*time.Second)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		Describe("cpuset controller", func() {
			It("Verify cpuset controller interface files have right values", Label("test1"), func() {
				var getter config.Getter
				getter, err := cgroup.BuildGetter(ctx)
				Expect(err).ToNot(HaveOccurred())
				containerCfg, err := getter.GetConfig(ctx, testclient.K8sClient, testpod, testpod.Spec.Containers[0].Name)
				cgroupCpuSet, err := cpuset.Parse(containerCfg.CPUSet)
				// Get cpus used by the container
				tasksetcmd := []string{"taskset", "-pc", "1"}
				testpodAffinity, err := pods.ExecCommandOnPod(testclient.K8sClient, testpod, testpod.Spec.Containers[0].Name, tasksetcmd)
				podCpusStr := string(testpodAffinity)
				parts := strings.Split(strings.TrimSpace(podCpusStr), ":")
				testpodCpus := strings.TrimSpace(parts[1])
				testlog.Infof("%v pod is using %v cpus", testpod.Name, string(testpodCpus))
				Expect(cgroupCpuSet.String()).To(Equal(testpodCpus), "cpuset.cpus not matching the process affinity")
			})
		})

		Describe("Load Balancing Annotation", func() {
			It("cpus used by container should not be load balanced", Label("test2"), func() {
				output, err := getPodCpus(testpod)
				Expect(err).ToNot(HaveOccurred(), "unable to fetch cpus used by testpod")
				podCpus, err := cpuset.Parse(output)
				Expect(err).ToNot(HaveOccurred(), "unable to parse cpuset used by pod")
				By("Getting the CPU scheduling flags")
				// After the testpod is started get the schedstat and check for cpus
				// not participating in scheduling domains
				checkSchedulingDomains(workerRTNode, podCpus, func(cpuIDs cpuset.CPUSet) error {
					if !podCpus.IsSubsetOf(cpuIDs) {
						return fmt.Errorf("pod CPUs NOT entirely part of cpus with load balance disabled: %v vs %v", podCpus, cpuIDs)
					}
					return nil
				}, 2*time.Minute, 5*time.Second, "checking scheduling domains with pod running")
			})
		})

		Describe("CPU Quota Annotation", func() {
			It("cpu controller interface files have correct values", func() {
				getter, err := cgroup.BuildGetter(ctx)
				Expect(err).ToNot(HaveOccurred())
				containerCfg, err := getter.GetConfig(ctx, testclient.K8sClient, testpod, testpod.Spec.Containers[0].Name)
				Expect(containerCfg.Quota).To(Equal("-1"))
			})

			It("checking new interface", Label("cgroupFun"), func() {
				c := &cgroups.CgroupBuilder{}
				podCgInfo := c.Node(context.TODO(), workerRTNode).Pod(testpod)
				output, err := podCgInfo.PodCgroupControllerInterface("cpu", "cpu.stat")
				fmt.Println(output)
				Expect(err).ToNot(HaveOccurred())

			})

		})

	})
})

// getCPUswithLoadBalanceDisabled Return cpus which are not in any scheduling domain
/*func getCPUswithLoadBalanceDisabled(targetNode *corev1.Node) ([]string, error) {
	cmd := []string{"/bin/bash", "-c", "cat /proc/schedstat"}
	schedstatData, err := nodes.ExecCommandOnNode(cmd, targetNode)
	if err != nil {
		return nil, err
	}

	info, err := schedstat.ParseData(strings.NewReader(schedstatData))
	if err != nil {
		return nil, err
	}

	cpusWithoutDomain := []string{}
	for _, cpu := range info.GetCPUs() {
		doms, ok := info.GetDomains(cpu)
		if !ok {
			return nil, fmt.Errorf("unknown cpu: %v", cpu)
		}
		if len(doms) > 0 {
			continue
		}
		cpusWithoutDomain = append(cpusWithoutDomain, cpu)
	}

	return cpusWithoutDomain, nil
    }*/

// getTestPodWithAnnotations Add node selector to test pod
/*func getTestPodWithAnnotations(annotations map[string]string, cpus int) *corev1.Pod {
	testpod := getTestPodWithProfileAndAnnotations(profile, annotations, cpus)

	testpod.Spec.NodeSelector = map[string]string{testutils.LabelHostname: workerRTNode.Name}

	return testpod
}
*/
// getTestPodWithProfileAndAnnotations Create pod with appropriate resources and annotations
/*func getTestPodWithProfileAndAnnotations(perfProf *performancev2.PerformanceProfile, annotations map[string]string, cpus int) *corev1.Pod {
	testpod := pods.GetTestPod()
	if len(annotations) > 0 {
		testpod.Annotations = annotations
	}
	testpod.Namespace = testutils.NamespaceTesting

	cpuCount := fmt.Sprintf("%d", cpus)

	resCpu := resource.MustParse(cpuCount)
	resMem := resource.MustParse("256Mi")

	// change pod resource requirements, to change the pod QoS class to guaranteed
	testpod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resCpu,
			corev1.ResourceMemory: resMem,
		},
	}

	if perfProf != nil {
		runtimeClassName := components.GetComponentName(perfProf.Name, components.ComponentNamePrefix)
		testpod.Spec.RuntimeClassName = &runtimeClassName
	}

	return testpod
}
*/
// getPodCpus return cpus used based on taskset
func getPodCpus(testpod *corev1.Pod) (string, error) {
	tasksetcmd := []string{"taskset", "-pc", "1"}
	testpodCpusByte, err := pods.ExecCommandOnPod(testclient.K8sClient, testpod, testpod.Spec.Containers[0].Name, tasksetcmd)
	if err != nil {
		return "", err
	}
	testpodCpusStr := string(testpodCpusByte)
	parts := strings.Split(strings.TrimSpace(testpodCpusStr), ":")
	cpus := strings.TrimSpace(parts[1])
	return cpus, err
}

// checkSchedulingDomains Check cpus are part of any scheduling domain
func checkSchedulingDomains(workerRTNode *corev1.Node, podCpus cpuset.CPUSet, testFunc func(cpuset.CPUSet) error, timeout, polling time.Duration, errMsg string) {
	Eventually(func() error {
		cpusNotInSchedulingDomains, err := getCPUswithLoadBalanceDisabled(workerRTNode)
		Expect(err).ToNot(HaveOccurred())
		testlog.Infof("cpus with load balancing disabled are: %v", cpusNotInSchedulingDomains)
		Expect(err).ToNot(HaveOccurred(), "unable to fetch cpus with load balancing disabled from /proc/schedstat")
		cpuIDList, err := schedstat.MakeCPUIDListFromCPUList(cpusNotInSchedulingDomains)
		if err != nil {
			return err
		}
		cpuIDs := cpuset.New(cpuIDList...)
		return testFunc(cpuIDs)
	}).WithTimeout(2*time.Minute).WithPolling(5*time.Second).ShouldNot(HaveOccurred(), errMsg)

}
