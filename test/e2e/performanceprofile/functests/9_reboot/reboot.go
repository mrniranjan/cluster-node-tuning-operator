package __reboot_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cpuset "github.com/openshift-kni/debug-tools/pkg/k8s_imported"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/cgroup"
	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/deployments"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/discovery"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/images"
	testlog "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/log"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/pods"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
)

const minRequiredCPUs = 8

var _ = Describe("[reboot] Long running tests", Label("kubepods"), Ordered, func() {
	const (
		cgroupRoot string = "/rootfs/sys/fs/cgroup"
	)
	var (
		//onlineCPUSet cpuset.CPUSet
		workerRTNode *corev1.Node
		//profile      *performancev2.PerformanceProfile
		isCgroupV2 bool
		ctx        context.Context = context.Background()
	)

	BeforeAll(func() {
		if discovery.Enabled() && testutils.ProfileNotFound {
			Skip("Discovery mode enabled, performance profile not found")
		}
		workerRTNodes, err := nodes.GetByLabels(testutils.NodeSelectorLabels)
		Expect(err).ToNot(HaveOccurred())

		workerRTNodes, err = nodes.MatchingOptionalSelector(workerRTNodes)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("error looking for the optional selector: %v", err))
		workerRTNode = &workerRTNodes[0]

		//profile, err = profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
		//Expect(err).ToNot(HaveOccurred())

		//performanceMCP, err = mcps.GetByProfile(profile)
		//Expect(err).ToNot(HaveOccurred())

		isCgroupV2, err = cgroup.IsVersion2(ctx, testclient.Client)
		Expect(err).ToNot(HaveOccurred())

		//onlineCPUSet, err = nodes.GetOnlineCPUsSet(context.TODO(), workerRTNode)
		//Expect(err).ToNot(HaveOccurred())

	})

	It("checking kubepods.slice", func() {
		checkCpuCount(context.TODO(), workerRTNode)
		var kubepodsCgroupPath string
		var dp *appsv1.Deployment
		// create a deployment to deploy gu pods
		dp = newDeployment()
		testNode := make(map[string]string)
		testNode["kubernetes.io/hostname"] = workerRTNode.Name
		dp.Spec.Template.Spec.NodeSelector = testNode
		err := testclient.Client.Create(ctx, dp)
		Expect(err).ToNot(HaveOccurred(), "Unable to create Deployment")

		defer func() {
			// delete deployment
			testlog.Infof("Deleting Deployment %v", dp.Name)
			err := testclient.Client.Delete(ctx, dp)
			Expect(err).ToNot(HaveOccurred())
		}()
		podList := &corev1.PodList{}
		listOptions := &client.ListOptions{Namespace: testutils.NamespaceTesting, LabelSelector: labels.SelectorFromSet(dp.Spec.Selector.MatchLabels)}
		Eventually(func() bool {
			isReady, err := deployments.IsReady(ctx, testclient.Client, listOptions, podList, dp)
			Expect(err).ToNot(HaveOccurred())
			return isReady
		}, time.Minute, time.Second).Should(BeTrue())
		Expect(testclient.Client.List(ctx, podList, listOptions)).To(Succeed())
		var cpulist []cpuset.CPUSet
		for _, pod := range podList.Items {
			cgroupCpusetCpus := ""
			ctnId, err := pods.GetContainerIDByName(&pod, pod.Spec.Containers[0].Name)
			Expect(err).ToNot(HaveOccurred())
			pid, err := nodes.ContainerPid(ctx, workerRTNode, ctnId)
			Expect(err).ToNot(HaveOccurred())
			cmd := []string{"cat", fmt.Sprintf("/rootfs/proc/%s/cgroup", pid)}
			out, err := nodes.ExecCommandOnMachineConfigDaemon(ctx, workerRTNode, cmd)
			Expect(err).ToNot(HaveOccurred())
			cgroupPathOfPid, err := cgroup.PidParser(out)
			if isCgroupV2 {
				cgroupCpusetCpus = fmt.Sprintf("%s%s/cpuset.cpus", cgroupRoot, cgroupPathOfPid)
			} else {
				cgroupCpusetCpus = fmt.Sprintf("%s/cpuset%s/cpuset.cpus", cgroupRoot, cgroupPathOfPid)
			}
			cmd = []string{"cat", cgroupCpusetCpus}
			cpus, err := nodes.ExecCommandOnNode(ctx, cmd, workerRTNode)
			ctnCpuset, err := cpuset.Parse(cpus)
			cpulist = append(cpulist, ctnCpuset)
			fmt.Println(cpulist)
		}
		if isCgroupV2 {
			kubepodsCgroupPath = fmt.Sprintf("%s/kubepods.slice/cpuset.cpus.exclusive", cgroupRoot)
		} else {
			kubepodsCgroupPath = fmt.Sprintf("%s/cpuset/kubepods.slice/cpuset.cpus.exclusive", cgroupRoot)
		}
		cmd := []string{"cat", kubepodsCgroupPath}
		cpus, err := nodes.ExecCommandOnNode(ctx, cmd, workerRTNode)
		Expect(err).ToNot(HaveOccurred())
		fmt.Println(cpus)

	})

})

// checkCpuCount check if the node has sufficient cpus
func checkCpuCount(ctx context.Context, workerNode *corev1.Node) {
	onlineCPUCount, err := nodes.ExecCommandOnNode(ctx, []string{"nproc", "--all"}, workerNode)
	if err != nil {
		Fail(fmt.Sprintf("Failed to fetch online CPUs: %v", err))
	}
	onlineCPUInt, err := strconv.Atoi(onlineCPUCount)
	if err != nil {
		Fail(fmt.Sprintf("failed to convert online CPU count to integer: %v", err))
	}
	if onlineCPUInt <= minRequiredCPUs {
		Skip(fmt.Sprintf("This test requires more than %d isolated CPUs, current available CPUs: %s", minRequiredCPUs, onlineCPUCount))
	}
}

// waitForCondition wait for deployment to be ready
func waitForCondition(deployment *appsv1.Deployment, status appsv1.DeploymentStatus) error {
	var err error
	var val bool
	err = wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err != nil {
			if errors.IsNotFound(err) {
				return false, fmt.Errorf("deployment not found")
			}
			return false, err
		}
		val = deployment.Status.Replicas == status.Replicas && deployment.Status.AvailableReplicas == status.AvailableReplicas
		return val, err
	})

	return err
}

func newDeployment() *appsv1.Deployment {
	var replicas int32 = 2
	dp := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Labels: map[string]string{
				"testDeployment": "",
			},
			Namespace: testutils.NamespaceTesting,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"type": "telco",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"type": "telco",
					},
					Annotations: map[string]string{
						"cpu-load-balancing.crio.io": "disable",
						"cpu-quota.crio.io":          "disable",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "test",
							Image:   images.Test(),
							Command: []string{"sleep", "inf"},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("200Mi"),
									corev1.ResourceCPU:    resource.MustParse("2"),
								},
							},
						},
					},
				},
			},
		},
	}
	return dp
}
