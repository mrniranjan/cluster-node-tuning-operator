package updatecpus

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUpdatecpus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Updatecpus Suite")
}
