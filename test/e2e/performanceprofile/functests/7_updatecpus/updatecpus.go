package updatecpus

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	performancev2 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/performanceprofile/v2"
	"k8s.io/utils/pointer"

	//"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profiles"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components"
	componentprofile "github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components/profile"
	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mcps"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profiles"
	machineconfigv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
)

var _ = Describe("Test MCP on OCP 4.11", func() {

	var workerRTNodes []corev1.Node
	var profile, initialProfile *performancev2.PerformanceProfile
	var err error
	profile, err = profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
	mcpLabel := componentprofile.GetMachineConfigLabel(profile)
	key, value := components.GetFirstKeyAndValue(mcpLabel)
	mcpsByLabel, err := mcps.GetByLabel(key, value)
	Expect(err).ToNot(HaveOccurred(), "Failed getting MCP by label key %v value %v", key, value)
	Expect(len(mcpsByLabel)).To(Equal(1), fmt.Sprintf("Unexpected number of MCPs found: %v", len(mcpsByLabel)))
	performanceMCP := &mcpsByLabel[0]
	Context("Modify Reserved and isolated cpus", func() {
		It("Test cpu mod", func() {
			workerRTNodes, err = nodes.GetByLabels(testutils.NodeSelectorLabels)
			Expect(err).ToNot(HaveOccurred())
			workerRTNodes, err = nodes.MatchingOptionalSelector(workerRTNodes)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("error looking for the optional selector: %v", err))
			Expect(workerRTNodes).ToNot(BeEmpty(), "cannot find RT enabled worker nodes")
			numaInfo, _ := nodes.GetNumaNodes(&workerRTNodes[0])
			cpuSlice := numaInfo[0][0:8]
			By("Modifying Profile")
			isolated := performancev2.CPUSet(fmt.Sprintf("%d-%d", cpuSlice[3], cpuSlice[7]))
			reserved := performancev2.CPUSet(fmt.Sprintf("%d-%d", cpuSlice[0], cpuSlice[2]))
			By("Modifying profile")
			initialProfile = profile.DeepCopy()

			profile.Spec.CPU = &performancev2.CPU{
				BalanceIsolated: pointer.BoolPtr(false),
				Reserved:        &reserved,
				Isolated:        &isolated,
			}

			By("Verifying that mcp is ready for update")
			mcps.WaitForCondition(performanceMCP.Name, machineconfigv1.MachineConfigPoolUpdated, corev1.ConditionTrue)
			spec, err := json.Marshal(profile.Spec)
			Expect(err).ToNot(HaveOccurred())

			By("Applying changes in performance profile and waiting until mcp will start updating")
			Expect(testclient.Client.Patch(context.TODO(), profile,
				client.RawPatch(
					types.JSONPatchType,
					[]byte(fmt.Sprintf(`[{ "op": "replace", "path": "/spec", "value": %s }]`, spec)),
				),
			)).ToNot(HaveOccurred())
			mcps.WaitForCondition(performanceMCP.Name, machineconfigv1.MachineConfigPoolUpdating, corev1.ConditionTrue)

			By("Waiting when mcp finishes updates")
			mcps.WaitForCondition(performanceMCP.Name, machineconfigv1.MachineConfigPoolUpdated, corev1.ConditionTrue)

		})

		It("Reverts back all profile configuration", func() {
			// return initial configuration
			spec, err := json.Marshal(initialProfile.Spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(testclient.Client.Patch(context.TODO(), profile,
				client.RawPatch(
					types.JSONPatchType,
					[]byte(fmt.Sprintf(`[{ "op": "replace", "path": "/spec", "value": %s }]`, spec)),
				),
			)).ToNot(HaveOccurred())
			mcps.WaitForCondition(performanceMCP.Name, machineconfigv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
			mcps.WaitForCondition(performanceMCP.Name, machineconfigv1.MachineConfigPoolUpdated, corev1.ConditionTrue)
		})

	})

})
