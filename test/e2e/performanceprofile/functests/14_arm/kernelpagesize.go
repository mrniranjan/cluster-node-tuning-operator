package __arm

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mcps"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/profiles"	
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/poolname"	
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/infrastructure"
	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"	
)

var _ = Describe("[rfe_id: 123343] Kernel Page size", func() {
	var (
		workerRTNodes           []corev1.Node
		perfProfile, initialProfile *performancev2.PerformanceProfile
		performanceMCP     string
		poolName                string
		err                     error
		ctx                     context.Context = context.Background()
		isArm             bool
		workerRTNode        corev1.Node
		
	)
	BeforeAll(func() {
		workerRTNodes, err = nodes.GetByLabels(testutils.NodeSelectorLabels)
		Expect(err).ToNot(HaveOccurred())

		workerRTNodes, err = nodes.MatchingOptionalSelector(workerRTNodes)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("error looking for the optional selector: %v", err))
		workerRTNode = workerRTNodes[0]

		perfProfile, err = profiles.GetByNodeLabels(testutils.NodeSelectorLabels)
		
		initialProfile = perfProfile.DeepCopy()
		poolName = poolname.GetByProfile(ctx, perfProfile)
		Expect(err).ToNot(HaveOccurred())

		isArm, err := infrastructure.IsARM(ctx, &workerRTNode)
		Expect(err).ToNot(HaveOccurred())
		
		if !isArm {
			Skip("Test requires ARM System")
		}
		By("Modifying the profile")
		perfProfile.Spec.
		
	})
	Context("64k page size", func() {	
		It("test 64k page size", func() {
			
		})
	})
})
			
			
			
	


