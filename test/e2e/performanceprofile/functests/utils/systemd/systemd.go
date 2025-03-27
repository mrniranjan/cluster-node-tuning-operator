package systemd

import (
	"context"
	"fmt"

	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	corev1 "k8s.io/api/core/v1"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mylog"
)

func Status(ctx context.Context, unitfile string, node *corev1.Node, logger *mylog.TestLogger) (string, error) {
	cmd := []string{"/bin/bash", "-c", fmt.Sprintf("chroot /rootfs systemctl status %s --lines=0 --no-pager", unitfile)}
	out, err := nodes.ExecCommand(ctx, node, cmd, logger)
	return string(out), err
}

func ShowProperty(ctx context.Context, unitfile string, property string, node *corev1.Node, logger *mylog.TestLogger) (string, error) {
	cmd := []string{"/bin/bash", "-c", fmt.Sprintf("chroot /rootfs systemctl show -p %s %s --no-pager", property, unitfile)}
	out, err := nodes.ExecCommand(ctx, node, cmd, logger)
	return string(out), err
}
