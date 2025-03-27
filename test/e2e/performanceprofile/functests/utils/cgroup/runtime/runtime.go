package runtime

import (
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mylog"
)

const (
	Crun                  = "crun"
	Runc                  = "runc"
	CRIORuntimeConfigFile = "/etc/crio/crio.conf.d/99-runtimes.conf"
)

// GetContainerRuntimeTypeFor return the container runtime type that is being used
// in the node where the given pod is running
func GetContainerRuntimeTypeFor(ctx context.Context, c client.Client, pod *corev1.Pod, logger *mylog.TestLogger) (string, error) {
	node := &corev1.Node{}
	if err := c.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, node); err != nil {
		return "", err
	}
	cmd := []string{
		"chroot",
		"/rootfs",
		"/bin/bash",
		"-c",
		fmt.Sprintf("/bin/ps aux | grep '%s' | grep -oP '(?<=-r\\s)[^\\s]+'", pod.Name),
	}
	output, err := nodes.ExecCommand(ctx, node, cmd, logger)
	if err != nil {
		return "", fmt.Errorf("failed to execute command on node; cmd=%q node=%q err=%v", cmd, node.Name, err)
	}
	out := testutils.ToString(output)
	return filepath.Base(out), nil
}
