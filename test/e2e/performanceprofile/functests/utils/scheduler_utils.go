package utils

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	testlog "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/log"
)

// SchedulabilityInfo contains information about control plane schedulability
type SchedulabilityInfo struct {
	// MastersSchedulableFromConfig indicates if mastersSchedulable is set to true in the Scheduler config
	MastersSchedulableFromConfig bool
	// ControlPlaneNodes is the list of control plane nodes found in the cluster
	ControlPlaneNodes []*corev1.Node
	// SchedulableControlPlaneNodes is the list of control plane nodes that are schedulable (no NoSchedule taint)
	SchedulableControlPlaneNodes []*corev1.Node
	// UnschedulableControlPlaneNodes is the list of control plane nodes that have NoSchedule taints
	UnschedulableControlPlaneNodes []*corev1.Node
}

// IsControlPlaneSchedulable checks if control plane nodes are schedulable using the OpenShift Scheduler configuration
// This checks the mastersSchedulable field in the cluster Scheduler config (config.openshift.io/v1)
func IsControlPlaneSchedulable(ctx context.Context) (bool, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return false, fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	configClient := clientconfigv1.NewForConfigOrDie(cfg)

	// Get the cluster scheduler configuration
	scheduler, err := configClient.Schedulers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get scheduler configuration: %v", err)
	}

	return scheduler.Spec.MastersSchedulable, nil
}

// GetSchedulerConfiguration retrieves the full OpenShift Scheduler configuration
func GetSchedulerConfiguration(ctx context.Context) (*configv1.Scheduler, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	configClient := clientconfigv1.NewForConfigOrDie(cfg)

	scheduler, err := configClient.Schedulers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduler configuration: %v", err)
	}

	return scheduler, nil
}

// GetControlPlaneSchedulabilityInfo provides comprehensive information about control plane schedulability
// This function checks both the Scheduler config and actual node taints to give a complete picture
func GetControlPlaneSchedulabilityInfo(ctx context.Context) (*SchedulabilityInfo, error) {
	info := &SchedulabilityInfo{}

	// Check mastersSchedulable from Scheduler config
	mastersSchedulable, err := IsControlPlaneSchedulable(ctx)
	if err != nil {
		testlog.Warningf("Failed to get mastersSchedulable from config: %v", err)
	}
	info.MastersSchedulableFromConfig = mastersSchedulable

	// Get all control plane nodes
	controlPlaneNodes, err := GetControlPlaneNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane nodes: %v", err)
	}
	info.ControlPlaneNodes = controlPlaneNodes

	// Categorize nodes based on schedulability (checking taints)
	for _, node := range controlPlaneNodes {
		if IsNodeSchedulable(node) {
			info.SchedulableControlPlaneNodes = append(info.SchedulableControlPlaneNodes, node)
		} else {
			info.UnschedulableControlPlaneNodes = append(info.UnschedulableControlPlaneNodes, node)
		}
	}

	return info, nil
}

// GetControlPlaneNodes returns all nodes with control plane role
func GetControlPlaneNodes(ctx context.Context) ([]*corev1.Node, error) {
	nodeList := &corev1.NodeList{}

	// Use label selector to find control plane nodes
	labelSelector := labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/control-plane": ""})
	listOptions := &client.ListOptions{
		LabelSelector: labelSelector,
	}

	if err := testclient.DataPlaneClient.List(ctx, nodeList, listOptions); err != nil {
		return nil, fmt.Errorf("failed to list control plane nodes: %v", err)
	}

	var nodes []*corev1.Node
	for i := range nodeList.Items {
		nodes = append(nodes, &nodeList.Items[i])
	}

	// Also check for legacy master label if no control-plane nodes found
	if len(nodes) == 0 {
		legacyLabelSelector := labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/master": ""})
		legacyListOptions := &client.ListOptions{
			LabelSelector: legacyLabelSelector,
		}

		if err := testclient.DataPlaneClient.List(ctx, nodeList, legacyListOptions); err != nil {
			return nil, fmt.Errorf("failed to list master nodes: %v", err)
		}

		for i := range nodeList.Items {
			nodes = append(nodes, &nodeList.Items[i])
		}
	}

	return nodes, nil
}

// IsNodeSchedulable checks if a node is schedulable by examining its taints
// A node is considered unschedulable if it has a NoSchedule taint for control plane scheduling
func IsNodeSchedulable(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		// Check for NoSchedule taints related to control plane
		if taint.Effect == corev1.TaintEffectNoSchedule {
			// Common control plane taints that make nodes unschedulable
			if taint.Key == "node-role.kubernetes.io/master" ||
				taint.Key == "node-role.kubernetes.io/control-plane" {
				return false
			}
		}
	}
	return true
}

// LogSchedulabilityInfo logs detailed information about control plane schedulability
func LogSchedulabilityInfo(ctx context.Context) error {
	info, err := GetControlPlaneSchedulabilityInfo(ctx)
	if err != nil {
		return err
	}

	testlog.Info("=== CONTROL PLANE SCHEDULABILITY INFO ===")
	testlog.Infof("mastersSchedulable in Scheduler config: %v", info.MastersSchedulableFromConfig)
	testlog.Infof("Total control plane nodes: %d", len(info.ControlPlaneNodes))
	testlog.Infof("Schedulable control plane nodes: %d", len(info.SchedulableControlPlaneNodes))
	testlog.Infof("Unschedulable control plane nodes: %d", len(info.UnschedulableControlPlaneNodes))

	if len(info.SchedulableControlPlaneNodes) > 0 {
		testlog.Info("Schedulable control plane nodes:")
		for _, node := range info.SchedulableControlPlaneNodes {
			testlog.Infof("  - %s", node.Name)
		}
	}

	if len(info.UnschedulableControlPlaneNodes) > 0 {
		testlog.Info("Unschedulable control plane nodes:")
		for _, node := range info.UnschedulableControlPlaneNodes {
			testlog.Infof("  - %s (taints: %v)", node.Name, node.Spec.Taints)
		}
	}

	// Check for inconsistencies
	hasSchedulableNodes := len(info.SchedulableControlPlaneNodes) > 0
	if info.MastersSchedulableFromConfig && !hasSchedulableNodes {
		testlog.Warning("⚠️  mastersSchedulable is true in config, but no control plane nodes appear schedulable")
	} else if !info.MastersSchedulableFromConfig && hasSchedulableNodes {
		testlog.Warning("⚠️  mastersSchedulable is false in config, but some control plane nodes appear schedulable")
	}

	return nil
}

// SetMastersSchedulable updates the mastersSchedulable field in the Scheduler configuration
func SetMastersSchedulable(ctx context.Context, schedulable bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	configClient := clientconfigv1.NewForConfigOrDie(cfg)

	// Get current scheduler configuration
	scheduler, err := configClient.Schedulers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get scheduler configuration: %v", err)
	}

	// Update the mastersSchedulable field
	scheduler.Spec.MastersSchedulable = schedulable

	// Update the configuration
	_, err = configClient.Schedulers().Update(ctx, scheduler, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update scheduler configuration: %v", err)
	}

	testlog.Infof("Successfully set mastersSchedulable to %v", schedulable)
	return nil
}
