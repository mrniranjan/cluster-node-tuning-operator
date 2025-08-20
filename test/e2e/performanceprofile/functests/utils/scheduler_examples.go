package utils

import (
	"context"
	"fmt"

	testlog "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/log"
)

// ExampleCheckControlPlaneSchedulability demonstrates how to check if control plane nodes are schedulable
func ExampleCheckControlPlaneSchedulability(ctx context.Context) {
	testlog.Info("=== Example: Checking Control Plane Schedulability ===")

	// Method 1: Check using OpenShift Scheduler configuration
	schedulable, err := IsControlPlaneSchedulable(ctx)
	if err != nil {
		testlog.Errorf("Failed to check control plane schedulability: %v", err)
		return
	}
	testlog.Infof("Control plane schedulable (from config): %v", schedulable)

	// Method 2: Get comprehensive information
	info, err := GetControlPlaneSchedulabilityInfo(ctx)
	if err != nil {
		testlog.Errorf("Failed to get schedulability info: %v", err)
		return
	}

	testlog.Infof("Total control plane nodes: %d", len(info.ControlPlaneNodes))
	testlog.Infof("Schedulable nodes: %d", len(info.SchedulableControlPlaneNodes))
	testlog.Infof("Unschedulable nodes: %d", len(info.UnschedulableControlPlaneNodes))

	// Method 3: Use the comprehensive logging function
	if err := LogSchedulabilityInfo(ctx); err != nil {
		testlog.Errorf("Failed to log schedulability info: %v", err)
	}
}

// ExampleEnableControlPlaneScheduling demonstrates how to make control plane nodes schedulable
func ExampleEnableControlPlaneScheduling(ctx context.Context) {
	testlog.Info("=== Example: Enabling Control Plane Scheduling ===")

	// Check current state
	currentState, err := IsControlPlaneSchedulable(ctx)
	if err != nil {
		testlog.Errorf("Failed to check current state: %v", err)
		return
	}
	testlog.Infof("Current mastersSchedulable state: %v", currentState)

	if !currentState {
		// Enable control plane scheduling
		testlog.Info("Enabling control plane scheduling...")
		if err := SetMastersSchedulable(ctx, true); err != nil {
			testlog.Errorf("Failed to enable control plane scheduling: %v", err)
			return
		}
		testlog.Info("✅ Control plane scheduling enabled")
	} else {
		testlog.Info("Control plane scheduling is already enabled")
	}

	// Verify the change
	newState, err := IsControlPlaneSchedulable(ctx)
	if err != nil {
		testlog.Errorf("Failed to verify change: %v", err)
		return
	}
	testlog.Infof("New mastersSchedulable state: %v", newState)
}

// ExampleDisableControlPlaneScheduling demonstrates how to make control plane nodes unschedulable
func ExampleDisableControlPlaneScheduling(ctx context.Context) {
	testlog.Info("=== Example: Disabling Control Plane Scheduling ===")

	// Check current state
	currentState, err := IsControlPlaneSchedulable(ctx)
	if err != nil {
		testlog.Errorf("Failed to check current state: %v", err)
		return
	}
	testlog.Infof("Current mastersSchedulable state: %v", currentState)

	if currentState {
		// Disable control plane scheduling
		testlog.Warning("⚠️  Disabling control plane scheduling - this will make control plane nodes unschedulable for workloads")
		if err := SetMastersSchedulable(ctx, false); err != nil {
			testlog.Errorf("Failed to disable control plane scheduling: %v", err)
			return
		}
		testlog.Info("✅ Control plane scheduling disabled")
	} else {
		testlog.Info("Control plane scheduling is already disabled")
	}

	// Verify the change
	newState, err := IsControlPlaneSchedulable(ctx)
	if err != nil {
		testlog.Errorf("Failed to verify change: %v", err)
		return
	}
	testlog.Infof("New mastersSchedulable state: %v", newState)
}

// ExampleGetSchedulerConfig demonstrates how to get the full scheduler configuration
func ExampleGetSchedulerConfig(ctx context.Context) {
	testlog.Info("=== Example: Getting Full Scheduler Configuration ===")

	scheduler, err := GetSchedulerConfiguration(ctx)
	if err != nil {
		testlog.Errorf("Failed to get scheduler configuration: %v", err)
		return
	}

	testlog.Infof("Scheduler name: %s", scheduler.Name)
	testlog.Infof("mastersSchedulable: %v", scheduler.Spec.MastersSchedulable)
	testlog.Infof("defaultNodeSelector: %s", scheduler.Spec.DefaultNodeSelector)

	if scheduler.Spec.Profile != "" {
		testlog.Infof("Scheduler profile: %s", scheduler.Spec.Profile)
	}

	fmt.Printf("Full scheduler config:\n%+v\n", scheduler.Spec)
}

// ExampleDetectSchedulabilityInconsistencies demonstrates how to detect mismatches between config and actual node state
func ExampleDetectSchedulabilityInconsistencies(ctx context.Context) {
	testlog.Info("=== Example: Detecting Schedulability Inconsistencies ===")

	info, err := GetControlPlaneSchedulabilityInfo(ctx)
	if err != nil {
		testlog.Errorf("Failed to get schedulability info: %v", err)
		return
	}

	// Check for inconsistencies
	configSaysSchedulable := info.MastersSchedulableFromConfig
	hasSchedulableNodes := len(info.SchedulableControlPlaneNodes) > 0
	hasUnschedulableNodes := len(info.UnschedulableControlPlaneNodes) > 0

	testlog.Infof("Config says schedulable: %v", configSaysSchedulable)
	testlog.Infof("Has schedulable nodes: %v", hasSchedulableNodes)
	testlog.Infof("Has unschedulable nodes: %v", hasUnschedulableNodes)

	if configSaysSchedulable && !hasSchedulableNodes {
		testlog.Warning("⚠️  INCONSISTENCY: Config enables scheduling but all nodes have NoSchedule taints")
		testlog.Info("This means the scheduler config allows scheduling, but node taints prevent it")
		testlog.Info("You may need to remove taints from control plane nodes")
	} else if !configSaysSchedulable && hasSchedulableNodes {
		testlog.Warning("⚠️  INCONSISTENCY: Config disables scheduling but some nodes lack NoSchedule taints")
		testlog.Info("This means some nodes could theoretically be schedulable despite config")
		testlog.Info("Consider adding taints or enabling mastersSchedulable")
	} else if configSaysSchedulable && hasSchedulableNodes {
		testlog.Info("✅ CONSISTENT: Config and node state both allow scheduling")
	} else {
		testlog.Info("✅ CONSISTENT: Config and node state both prevent scheduling")
	}
}

// ExampleInspectIndividualNodes demonstrates how to check individual control plane nodes
func ExampleInspectIndividualNodes(ctx context.Context) {
	testlog.Info("=== Example: Inspecting Individual Control Plane Nodes ===")

	nodes, err := GetControlPlaneNodes(ctx)
	if err != nil {
		testlog.Errorf("Failed to get control plane nodes: %v", err)
		return
	}

	testlog.Infof("Found %d control plane nodes:", len(nodes))

	for i, node := range nodes {
		testlog.Infof("\nNode %d: %s", i+1, node.Name)
		testlog.Infof("  Schedulable: %v", IsNodeSchedulable(node))

		// Show labels
		testlog.Info("  Labels:")
		for key, value := range node.Labels {
			if key == "node-role.kubernetes.io/control-plane" ||
				key == "node-role.kubernetes.io/master" {
				testlog.Infof("    %s: %s", key, value)
			}
		}

		// Show taints
		if len(node.Spec.Taints) > 0 {
			testlog.Info("  Taints:")
			for _, taint := range node.Spec.Taints {
				testlog.Infof("    %s=%s:%s", taint.Key, taint.Value, taint.Effect)
			}
		} else {
			testlog.Info("  Taints: none")
		}
	}
}

// ExampleCompleteWorkflow demonstrates a complete workflow for managing control plane schedulability
func ExampleCompleteWorkflow(ctx context.Context) {
	testlog.Info("=== Example: Complete Control Plane Schedulability Workflow ===")

	// Step 1: Get current state
	testlog.Info("Step 1: Getting current state...")
	if err := LogSchedulabilityInfo(ctx); err != nil {
		testlog.Errorf("Failed to get current state: %v", err)
		return
	}

	// Step 2: Check for inconsistencies
	testlog.Info("\nStep 2: Checking for inconsistencies...")
	ExampleDetectSchedulabilityInconsistencies(ctx)

	// Step 3: Demonstrate enabling scheduling (if needed)
	schedulable, err := IsControlPlaneSchedulable(ctx)
	if err != nil {
		testlog.Errorf("Failed to check schedulability: %v", err)
		return
	}

	if !schedulable {
		testlog.Info("\nStep 3: Enabling control plane scheduling...")
		ExampleEnableControlPlaneScheduling(ctx)

		testlog.Info("\nStep 4: Verifying changes...")
		if err := LogSchedulabilityInfo(ctx); err != nil {
			testlog.Errorf("Failed to verify changes: %v", err)
			return
		}
	} else {
		testlog.Info("\nStep 3: Control plane scheduling already enabled, skipping...")
	}

	testlog.Info("\n✅ Workflow completed successfully!")
	testlog.Info("Note: In production, you would typically restore the original configuration")
	testlog.Info("after testing to maintain cluster security and performance.")
}
