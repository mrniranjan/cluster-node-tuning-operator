package __performance_ppc

import (
	"fmt"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	testlog "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/log"
)

const (
	podmanBinary = "/usr/bin/podman"
)

var DefaultNtoImage string
var MustgatherDir string

var _ = BeforeSuite(func() {
	var err error
	By("Check podman binary exists")
	path, err := exec.LookPath(podmanBinary)
	if err != nil {
		Skip(fmt.Sprintf("%s doesn't exists", podmanBinary))
	}
	testlog.Infof("Podman binary executed from path %s", path)
	By("Checking Environment Variables")
	testlog.Infof("NTO Image used: %s", testutils.NtoImageRegistry)
	if testutils.MustGatherDir == "" {
		Skip("set env variable MUSTGATHER_DIR to ocp mustgather directory")
	}
	testlog.Infof("Mustgather Directory used: %s", MustgatherDir)

})

func Test9Ppc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PPC Suite")
}