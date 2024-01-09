package cgroups

import (
	"context"
	"fmt"
	"strings"

	apiconfigv1 "github.com/openshift/api/config/v1"
	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mcps"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/pods"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CgroupInfo struct {
	Ctx      context.Context
	Pod      *corev1.Pod
	Cgroupv2 bool
	Node     *corev1.Node
	Runtime  string // crun or runc
}

type CgroupBuilder struct {
	cg CgroupInfo
}

func (c *CgroupBuilder) Builder() CgroupInfo {
	return (c.cg)
}

func (c *CgroupBuilder) Pod(testpod *corev1.Pod) *CgroupBuilder {
	c.cg.Pod = testpod
	return c
}

func (c *CgroupBuilder) Node(ctx context.Context, workerNode *corev1.Node) *CgroupBuilder {
	c.cg.Node = workerNode
	c.cg.Ctx = ctx
	return c
}

func (c *CgroupBuilder) CgroupVersion() error {
	nodecfg := &apiconfigv1.Node{}
	key := client.ObjectKey{
		Name: "cluster",
	}
	err := testclient.Client.Get(c.cg.Ctx, key, nodecfg)
	if err != nil {
		return fmt.Errorf("failed to get configs.node object name=%q; %w", key.Name, err)
	}
	if nodecfg.Spec.CgroupMode == apiconfigv1.CgroupModeV2 {
		c.cg.Cgroupv2 = true
	} else {
		c.cg.Cgroupv2 = false
	}
	return nil
}

func (c *CgroupBuilder) Runtime() error {
	val, err := mcps.GetValueFromCrioConfig(c.cg.Node, "runtime_path")
	if err != nil {
		return fmt.Errorf("failed to runtime path from crio config")
	}
	if strings.Contains(val[0], "crun") {
		c.cg.Runtime = "crun"
	} else if strings.Contains(val[0], "runc") {
		c.cg.Runtime = "runc"
	}
	return nil

}

func (c *CgroupBuilder) PodCgroupControllerInterface(controller, controllerInterfacefile string) ([]string, error) {
	var interfacePath string
	err := c.CgroupVersion()
	if err != nil {
		return nil, err
	}
	err = c.Runtime()
	if err != nil {
		return nil, err
	}

	if c.cg.Cgroupv2 && c.cg.Runtime == "runc" {
		interfacePath = fmt.Sprintf("/sys/fs/cgroup/%s", controllerInterfacefile)
	} else if c.cg.Cgroupv2 && c.cg.Runtime == "crun" {
		interfacePath = fmt.Sprintf("/sys/fs/cgroup/container/%s", controllerInterfacefile)
	} else if !c.cg.Cgroupv2 && c.cg.Runtime == "runc" {
		interfacePath = fmt.Sprintf("/sys/fs/cgroup/%s/%s", controller, controllerInterfacefile)
	} else if !c.cg.Cgroupv2 && c.cg.Runtime == "crun" {
		interfacePath = fmt.Sprintf("/sys/fs/cgroup/%s/container/%s", controller, controllerInterfacefile)
	}
	cmd := []string{"/bin/cat", interfacePath}
	byteoutput, err := pods.ExecCommandOnPod(testclient.K8sClient, c.cg.Pod, c.cg.Pod.Spec.Containers[0].Name, cmd)
	output := strings.Split(string(byteoutput), "\r\n")
	return output, err
}
