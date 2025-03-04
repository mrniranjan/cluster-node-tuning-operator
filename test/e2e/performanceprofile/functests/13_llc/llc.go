package __llc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
	"k8s.io/utils/cpuset"
	"strings"
	
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/labels"
	
	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	performancev2 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/performanceprofile/v2"
	profilecomponent "github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components/profile"
	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/cgroup"
	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/label"
	testlog "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/log"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/pods"

	//"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mcps"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/cgroup/controller"
	//"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/images"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/namespaces"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/poolname"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profiles"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profilesupdate"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/deployments"
)

const (
	llcEnableFileName            = "/etc/kubernetes/openshift-llc-alignment"
	defaultIgnitionContentSource = "data:text/plain;charset=utf-8;base64"
	defaultIgnitionVersion       = "3.2.0"
	fileMode                     = 0420
)

var _ = Describe("[rfe_id:77446] LLC-aware cpu pinning", Label(string(label.OpenShift)), Ordered, func() {
	var (
		workerRTNodes      []corev1.Node
		perfProfile        *performancev2.PerformanceProfile
		//performanceMCP     string
		err                error
		profileAnnotations map[string]string
		poolName           string
		//llcPolicy          string
		//mc                 *machineconfigv1.MachineConfig
		getter                   cgroup.ControllersGetter
		cgroupV2                 bool
	)

	BeforeAll(func() {
		profileAnnotations = make(map[string]string)
		ctx := context.Background()

		workerRTNodes, err = nodes.GetByLabels(testutils.NodeSelectorLabels)
		Expect(err).ToNot(HaveOccurred())

		workerRTNodes, err = nodes.MatchingOptionalSelector(workerRTNodes)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("error looking for the optional selector: %v", err))

		perfProfile, err = profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
		Expect(err).ToNot(HaveOccurred())

		getter, err = cgroup.BuildGetter(ctx, testclient.DataPlaneClient, testclient.K8sClient)
		Expect(err).ToNot(HaveOccurred())
		cgroupV2, err = cgroup.IsVersion2(ctx, testclient.DataPlaneClient)
		Expect(err).ToNot(HaveOccurred())
		if !cgroupV2 {
			Skip("prefer-align-cpus-by-uncorecache cpumanager policy options is supported in cgroupv2 configuration only")
		}
		//performanceMCP, err = mcps.GetByProfile(perfProfile)
		//Expect(err).ToNot(HaveOccurred())

		poolName = poolname.GetByProfile(ctx, perfProfile)

		/*mc, err = createMachineConfig(perfProfile)
		Expect(err).ToNot(HaveOccurred())

		llcPolicy = `{"cpuManagerPolicyOptions":{"prefer-align-cpus-by-uncorecache":"true", "full-pcpus-only":"true"}}`
		profileAnnotations["kubeletconfig.experimental"] = llcPolicy

		// Create machine config to create file /etc/kubernetes/openshift-llc-alignment
		// required to enable align-cpus-by-uncorecache cpumanager policy

		By("Enabling Uncore cache feature")
		Expect(testclient.Client.Create(context.TODO(), mc)).To(Succeed(), "Unable to apply machine config for enabling uncore cache")

		Expect(err).ToNot(HaveOccurred(), "Unable to apply machine config for enabling uncore cache")

		mcps.WaitForCondition(performanceMCP, machineconfigv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
		By("Waiting when mcp finishes updates")

		mcps.WaitForCondition(performanceMCP, machineconfigv1.MachineConfigPoolUpdated, corev1.ConditionTrue)

		// Apply Annotation to enable align-cpu-by-uncorecache cpumanager policy option
		if perfProfile.Annotations == nil || perfProfile.Annotations["kubeletconfig.experimental"] != llcPolicy {
			testlog.Info("Enable align-cpus-by-uncorecache cpumanager policy")
			perfProfile.Annotations = profileAnnotations

			By("updating performance profile")
			profiles.UpdateWithRetry(perfProfile)

			By(fmt.Sprintf("Applying changes in performance profile and waiting until %s will start updating", poolName))
			profilesupdate.WaitForTuningUpdating(ctx, perfProfile)

			By(fmt.Sprintf("Waiting when %s finishes updates", poolName))
			profilesupdate.WaitForTuningUpdated(ctx, perfProfile)
		}*/

	})

	AfterAll(func() {

		// Delete machine config created to enable uncocre cache cpumanager policy option
		// first make sure the profile doesn't have the annotation
		fmt.Println("We are in afterall and nothing is deleted")
		/*ctx := context.Background()
		perfProfile, err = profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
		perfProfile.Annotations = nil
		By("updating performance profile")
		profiles.UpdateWithRetry(perfProfile)

		By(fmt.Sprintf("Applying changes in performance profile and waiting until %s will start updating", poolName))
		profilesupdate.WaitForTuningUpdating(ctx, perfProfile)

		By(fmt.Sprintf("Waiting when %s finishes updates", poolName))
		profilesupdate.WaitForTuningUpdated(ctx, perfProfile)

		// delete the machine config pool
		Expect(testclient.Client.Delete(ctx, mc)).To(Succeed())

		mcps.WaitForCondition(performanceMCP, machineconfigv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
		By("Waiting when mcp finishes updates")

		mcps.WaitForCondition(performanceMCP, machineconfigv1.MachineConfigPoolUpdated, corev1.ConditionTrue)*/
	})

	Context("Configuration Tests", func() {
		When("align-cpus-by-uncorecache cpumanager policy option is enabled", func() {
			It("[test_id:77722] kubelet is configured appropriately", func() {
				ctx := context.Background()
				for _, node := range workerRTNodes {
					kubeletconfig, err := nodes.GetKubeletConfig(ctx, &node)
					Expect(err).ToNot(HaveOccurred())
					Expect(kubeletconfig.CPUManagerPolicyOptions).To(HaveKeyWithValue("prefer-align-cpus-by-uncorecache", "true"))
				}
			})
		})

		When("align-cpus-by-uncorecache annotations is removed", func() {
			It("[test_id:77723] should disable align-cpus-by-uncorecache cpumanager policy option", func() {
				ctx := context.Background()
				// Delete the Annotations
				if perfProfile.Annotations != nil {
					perfProfile.Annotations = nil

					By("updating performance profile")
					profiles.UpdateWithRetry(perfProfile)

					By(fmt.Sprintf("Applying changes in performance profile and waiting until %s will start updating", poolName))
					profilesupdate.WaitForTuningUpdating(ctx, perfProfile)

					By(fmt.Sprintf("Waiting when %s finishes updates", poolName))
					profilesupdate.WaitForTuningUpdated(ctx, perfProfile)
				}

				for _, node := range workerRTNodes {
					kubeletconfig, err := nodes.GetKubeletConfig(ctx, &node)
					Expect(err).ToNot(HaveOccurred())
					Expect(kubeletconfig.CPUManagerPolicyOptions).ToNot(HaveKey("prefer-align-cpus-by-uncorecache"))
				}
			})
		})

		When("align-cpus-by-uncorecache cpumanager policy option is disabled", func() {
			It("[test_id:77724] cpumanager Policy option in kubelet is configured appropriately", func() {
				ctx := context.Background()
				llcDisablePolicy := `{"cpuManagerPolicyOptions":{"prefer-align-cpus-by-uncorecache":"false", "full-pcpus-only":"true"}}`
				profileAnnotations["kubeletconfig.experimental"] = llcDisablePolicy
				perfProfile.Annotations = profileAnnotations

				By("updating performance profile")
				profiles.UpdateWithRetry(perfProfile)

				By(fmt.Sprintf("Applying changes in performance profile and waiting until %s will start updating", poolName))
				profilesupdate.WaitForTuningUpdating(ctx, perfProfile)

				By(fmt.Sprintf("Waiting when %s finishes updates", poolName))
				profilesupdate.WaitForTuningUpdated(ctx, perfProfile)

				for _, node := range workerRTNodes {
					kubeletconfig, err := nodes.GetKubeletConfig(ctx, &node)
					Expect(err).ToNot(HaveOccurred())
					Expect(kubeletconfig.CPUManagerPolicyOptions).To(HaveKeyWithValue("prefer-align-cpus-by-uncorecache", "false"))
				}
			})
		})
	})
	Context("Functional Tests", func() {
		BeforeAll(func() {
			var cpuGroupSize int
			ctx := context.Background()
			
			for _, cnfnode := range workerRTNodes {
				numaInfo, err := nodes.GetNumaNodes(context.TODO(), &cnfnode)
				Expect(err).ToNot(HaveOccurred(), "Unable to get numa information from the node")
				if len(numaInfo) < 2 {
					Skip(fmt.Sprintf("This test need 2 Numa nodes. The number of numa nodes on node %s < 2", cnfnode.Name))
				}	
				uncoreCpusOfCpuZero := UnCoreCacheCpus(&cnfnode)
				cpus, err := uncoreCpusOfCpuZero(0)
				Expect(err).ToNot(HaveOccurred())
				cpuGroupSize = cpus.Size()
				if len(numaInfo[0]) == cpuGroupSize {
					Skip("This test requires systems where L3 cache is shared amount subset of cpus")
				}
			}
			testNS := *namespaces.TestingNamespace
			Expect(testclient.DataPlaneClient.Create(ctx, &testNS)).ToNot(HaveOccurred())
			DeferCleanup(func() {
				Expect(testclient.DataPlaneClient.Delete(ctx, &testNS)).ToNot(HaveOccurred())
				Expect(namespaces.WaitForDeletion(testutils.NamespaceTesting, 5*time.Minute)).ToNot(HaveOccurred())
			})			
		})
		
		It("[test_id:77725] Align Guaranteed pod requesting 16 cpus to the whole CCX if available", Label("llc1"), func() {
			ctx := context.Background()
			cpusetCfg := &controller.CpuSet{}
			DeploymentName := "test-deployment"
			By("Creating a deployment with one pod asking for whole L3 cache group")
			rl := &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			}
			p := makePod(testutils.NamespaceTesting, withRequests(rl),withLimits(rl))
			dp := deployments.Make(DeploymentName, testutils.NamespaceTesting,
				deployments.WithPodTemplate(p),
				deployments.WithNodeSelector(testutils.NodeSelectorLabels))
			Expect(testclient.Client.Create(ctx, dp)).ToNot(HaveOccurred())
			podList := &corev1.PodList{}
			listOptions := &client.ListOptions{Namespace: testutils.NamespaceTesting, LabelSelector: labels.SelectorFromSet(dp.Spec.Selector.MatchLabels)}
			Eventually(func() bool {
				isReady, err := deployments.IsReady(ctx, testclient.Client, listOptions, podList, dp)
				Expect(err).ToNot(HaveOccurred())
				return isReady
			},  time.Minute, time.Second).Should(BeTrue())
			Expect(testclient.Client.List(ctx, podList, listOptions)).To(Succeed())
			Expect(len(podList.Items)).To(Equal(1), "Expected exactly one pod in the list")
			testpod := podList.Items[0]
			err := getter.Container(ctx, &testpod, testpod.Spec.Containers[0].Name, cpusetCfg)
			Expect(err).ToNot(HaveOccurred())
			cgroupCpuset, err := cpuset.Parse(cpusetCfg.Cpus)
			Expect(err).ToNot(HaveOccurred())
			fmt.Println(cgroupCpuset.List())
			uncoreCpuGroups := UnCoreCacheCpus(&workerRTNodes[0])
			cpus, err := uncoreCpuGroups(cgroupCpuset.List()[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(cgroupCpuset).To(Equal(cpus))
			defer func() {
				// delete deployment
				testlog.Infof("Deleting Deployment %v", DeploymentName)
				err := testclient.DataPlaneClient.Delete(ctx, dp)
				Expect(err).ToNot(HaveOccurred())
			}()
		})
	})
})

// create Machine config to create text file required to enable prefer-align-cpus-by-uncorecache policy option
func createMachineConfig(perfProfile *performancev2.PerformanceProfile) (*machineconfigv1.MachineConfig, error) {
	mcName := "openshift-llc-alignment"
	mc := &machineconfigv1.MachineConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: machineconfigv1.GroupVersion.String(),
			Kind:       "MachineConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   mcName,
			Labels: profilecomponent.GetMachineConfigLabel(perfProfile),
		},
		Spec: machineconfigv1.MachineConfigSpec{},
	}
	ignitionConfig := &igntypes.Config{
		Ignition: igntypes.Ignition{
			Version: defaultIgnitionVersion,
		},
		Storage: igntypes.Storage{
			Files: []igntypes.File{},
		},
	}
	addContent(ignitionConfig, []byte("enabled"), llcEnableFileName, fileMode)
	rawIgnition, err := json.Marshal(ignitionConfig)
	if err != nil {
		return nil, err
	}
	mc.Spec.Config = runtime.RawExtension{Raw: rawIgnition}
	return mc, nil
}

// creates the ignitionConfig file
func addContent(ignitionConfig *igntypes.Config, content []byte, dst string, mode int) {
	contentBase64 := base64.StdEncoding.EncodeToString(content)
	ignitionConfig.Storage.Files = append(ignitionConfig.Storage.Files, igntypes.File{
		Node: igntypes.Node{
			Path: dst,
		},
		FileEmbedded1: igntypes.FileEmbedded1{
			Contents: igntypes.Resource{
				Source: ptr.To(fmt.Sprintf("%s,%s", defaultIgnitionContentSource, contentBase64)),
			},
			Mode: &mode,
		},
	})
}


func makePod(ns string, opts ...func(pod *corev1.Pod)) *corev1.Pod {
	p := pods.GetTestPod()
	p.Namespace = ns
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func withRequests(rl *corev1.ResourceList) func(p *corev1.Pod) {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Resources.Requests = *rl
	}
}

func withLimits(rl *corev1.ResourceList) func(p *corev1.Pod) {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Resources.Limits = *rl
	}
}

func UnCoreCacheCpus(node *corev1.Node) func(cpuId int) (cpuset.CPUSet, error) {
	return func (cpuId int) (cpuset.CPUSet, error) {
		cacheSizeFile := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cache/index3/shared_cpu_list", cpuId)
		cmd := []string{"cat", cacheSizeFile}
		ctx := context.Background()
		output, err := nodes.ExecCommand(ctx, node, cmd)
		if err != nil {
			return cpuset.CPUSet{}, fmt.Errorf("Unable to fetch shared cpu list: %v", err)
		}
		cpuSet, err := cpuset.Parse(strings.TrimSpace(string(output)))
		return cpuSet, err
	}
}
