# Control Plane Schedulability Utilities

This document explains how to check if control plane nodes are schedulable in an OpenShift cluster and how to configure schedulability.

## Overview

Control plane nodes in OpenShift are by default not schedulable for regular workloads. This is controlled by two mechanisms:

1. **OpenShift Scheduler Configuration**: The `mastersSchedulable` field in the cluster Scheduler configuration (`config.openshift.io/v1`)
2. **Node Taints**: Control plane nodes typically have `NoSchedule` taints that prevent regular pods from being scheduled on them

The utilities in `scheduler_utils.go` provide functions to check and configure both aspects.

## Available Functions

### `IsControlPlaneSchedulable(ctx context.Context) (bool, error)`

Checks if control plane nodes are schedulable by reading the `mastersSchedulable` field from the OpenShift Scheduler configuration.

```go
schedulable, err := IsControlPlaneSchedulable(ctx)
if err != nil {
    log.Errorf("Failed to check schedulability: %v", err)
    return
}
log.Infof("Control plane schedulable: %v", schedulable)
```

### `GetControlPlaneSchedulabilityInfo(ctx context.Context) (*SchedulabilityInfo, error)`

Provides comprehensive information about control plane schedulability, including:
- The `mastersSchedulable` setting from config
- List of all control plane nodes
- Which nodes are actually schedulable (no `NoSchedule` taints)
- Which nodes are unschedulable (have `NoSchedule` taints)

```go
info, err := GetControlPlaneSchedulabilityInfo(ctx)
if err != nil {
    log.Errorf("Failed to get info: %v", err)
    return
}

log.Infof("Config says schedulable: %v", info.MastersSchedulableFromConfig)
log.Infof("Total control plane nodes: %d", len(info.ControlPlaneNodes))
log.Infof("Actually schedulable nodes: %d", len(info.SchedulableControlPlaneNodes))
```

### `LogSchedulabilityInfo(ctx context.Context) error`

Logs detailed information about control plane schedulability and checks for inconsistencies between configuration and actual node state.

```go
if err := LogSchedulabilityInfo(ctx); err != nil {
    log.Errorf("Failed to log info: %v", err)
}
```

### `SetMastersSchedulable(ctx context.Context, schedulable bool) error`

Updates the `mastersSchedulable` field in the OpenShift Scheduler configuration.

```go
// Enable control plane scheduling
if err := SetMastersSchedulable(ctx, true); err != nil {
    log.Errorf("Failed to enable scheduling: %v", err)
    return
}

// Disable control plane scheduling
if err := SetMastersSchedulable(ctx, false); err != nil {
    log.Errorf("Failed to disable scheduling: %v", err)
    return
}
```

### `GetSchedulerConfiguration(ctx context.Context) (*configv1.Scheduler, error)`

Retrieves the complete OpenShift Scheduler configuration object.

```go
scheduler, err := GetSchedulerConfiguration(ctx)
if err != nil {
    log.Errorf("Failed to get config: %v", err)
    return
}

log.Infof("mastersSchedulable: %v", scheduler.Spec.MastersSchedulable)
log.Infof("defaultNodeSelector: %s", scheduler.Spec.DefaultNodeSelector)
```

### `GetControlPlaneNodes(ctx context.Context) ([]*corev1.Node, error)`

Returns all nodes with control plane role (both `node-role.kubernetes.io/control-plane` and legacy `node-role.kubernetes.io/master`).

```go
nodes, err := GetControlPlaneNodes(ctx)
if err != nil {
    log.Errorf("Failed to get nodes: %v", err)
    return
}

for _, node := range nodes {
    log.Infof("Control plane node: %s", node.Name)
}
```

### `IsNodeSchedulable(node *corev1.Node) bool`

Checks if an individual node is schedulable by examining its taints.

```go
for _, node := range nodes {
    schedulable := IsNodeSchedulable(node)
    log.Infof("Node %s schedulable: %v", node.Name, schedulable)
}
```

## Understanding Control Plane Schedulability

### The Two Levels of Control

1. **Scheduler Configuration (`mastersSchedulable`)**
   - This is a cluster-wide setting in the Scheduler configuration
   - When `false` (default), the scheduler will not schedule regular workloads on control plane nodes
   - When `true`, the scheduler allows regular workloads on control plane nodes

2. **Node Taints**
   - Individual control plane nodes may have `NoSchedule` taints
   - Common taint keys:
     - `node-role.kubernetes.io/master`
     - `node-role.kubernetes.io/control-plane`
   - These taints prevent pods without matching tolerations from being scheduled

### Typical Scenarios

#### Scenario 1: Default Configuration (Control Plane Not Schedulable)
- `mastersSchedulable: false` in Scheduler config
- Control plane nodes have `NoSchedule` taints
- Regular workloads cannot be scheduled on control plane nodes

#### Scenario 2: Control Plane Made Schedulable
- `mastersSchedulable: true` in Scheduler config
- Control plane nodes may still have taints (depends on how it was configured)
- Regular workloads can potentially be scheduled on control plane nodes

#### Scenario 3: Inconsistent State (Warning)
- `mastersSchedulable: true` but nodes still have `NoSchedule` taints
- OR `mastersSchedulable: false` but some nodes don't have taints
- This indicates a configuration inconsistency

## Example Usage

### Simple Check
```go
package main

import (
    "context"
    "fmt"
    
    utils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
)

func main() {
    ctx := context.Background()
    
    // Quick check
    schedulable, err := utils.IsControlPlaneSchedulable(ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Control plane schedulable: %v\n", schedulable)
}
```

### Comprehensive Analysis
```go
func main() {
    ctx := context.Background()
    
    // Get detailed information
    utils.LogSchedulabilityInfo(ctx)
    
    // Check for inconsistencies
    utils.ExampleDetectSchedulabilityInconsistencies(ctx)
    
    // Inspect individual nodes
    utils.ExampleInspectIndividualNodes(ctx)
}
```

### Enable/Disable Scheduling
```go
func main() {
    ctx := context.Background()
    
    // Enable control plane scheduling
    utils.ExampleEnableControlPlaneScheduling(ctx)
    
    // Or disable it
    // utils.ExampleDisableControlPlaneScheduling(ctx)
}
```

## YAML Configuration Equivalent

The `SetMastersSchedulable` function is equivalent to applying this YAML:

```yaml
apiVersion: config.openshift.io/v1
kind: Scheduler
metadata:
  name: cluster
spec:
  mastersSchedulable: true  # or false
```

## Example Output

When you run `LogSchedulabilityInfo(ctx)`, you might see output like:

```
=== CONTROL PLANE SCHEDULABILITY INFO ===
mastersSchedulable in Scheduler config: false
Total control plane nodes: 3
Schedulable control plane nodes: 0
Unschedulable control plane nodes: 3
Unschedulable control plane nodes:
  - master-0 (taints: [node-role.kubernetes.io/master=:NoSchedule])
  - master-1 (taints: [node-role.kubernetes.io/master=:NoSchedule])
  - master-2 (taints: [node-role.kubernetes.io/master=:NoSchedule])
```

Or with inconsistencies:

```
=== CONTROL PLANE SCHEDULABILITY INFO ===
mastersSchedulable in Scheduler config: true
Total control plane nodes: 3
Schedulable control plane nodes: 1
Unschedulable control plane nodes: 2
Schedulable control plane nodes:
  - master-0
Unschedulable control plane nodes:
  - master-1 (taints: [node-role.kubernetes.io/master=:NoSchedule])
  - master-2 (taints: [node-role.kubernetes.io/master=:NoSchedule])
⚠️  mastersSchedulable is true in config, but some control plane nodes still appear unschedulable
```

## Available Examples

The `scheduler_examples.go` file contains several example functions:

- `ExampleCheckControlPlaneSchedulability(ctx)` - Basic checking
- `ExampleEnableControlPlaneScheduling(ctx)` - Enable scheduling
- `ExampleDisableControlPlaneScheduling(ctx)` - Disable scheduling
- `ExampleGetSchedulerConfig(ctx)` - Get full config
- `ExampleDetectSchedulabilityInconsistencies(ctx)` - Find mismatches
- `ExampleInspectIndividualNodes(ctx)` - Node-by-node analysis
- `ExampleCompleteWorkflow(ctx)` - Complete end-to-end workflow

## Important Notes

1. **Backup Configuration**: Always backup your current scheduler configuration before making changes
2. **Impact on Control Plane**: Enabling control plane scheduling allows regular workloads to run on control plane nodes, which may impact cluster performance
3. **Resource Contention**: Control plane components need resources; be careful about resource contention
4. **Taints vs Configuration**: Setting `mastersSchedulable: true` doesn't automatically remove node taints - those may need to be removed separately
5. **Testing**: Always test configuration changes in a non-production environment first

## Security Considerations

- Control plane nodes run critical cluster components
- Allowing regular workloads on control plane nodes can be a security risk
- Consider the impact of resource contention on cluster stability
- Use node selectors and resource limits when scheduling on control plane nodes

## Troubleshooting

### Common Issues

1. **Function returns error about missing scheduler config**
   - Ensure you have proper RBAC permissions to read cluster configuration
   - Check if the cluster has a valid scheduler configuration

2. **mastersSchedulable is true but nodes still unschedulable**
   - Check if control plane nodes have `NoSchedule` taints
   - Use `GetControlPlaneSchedulabilityInfo` to see the detailed state

3. **Changes not taking effect**
   - Scheduler configuration changes may take time to propagate
   - Check the scheduler operator logs for any errors

4. **Permission denied errors**
   - Ensure your service account has the necessary RBAC permissions
   - You need read/write access to `schedulers.config.openshift.io`

### Debug Commands

```bash
# Check current scheduler configuration
oc get scheduler cluster -o yaml

# Check control plane node taints
oc get nodes -l node-role.kubernetes.io/control-plane -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints

# Check control plane node labels
oc get nodes -l node-role.kubernetes.io/control-plane --show-labels

# Check scheduler operator logs
oc logs -n openshift-kube-scheduler-operator deployment/openshift-kube-scheduler-operator

# Check actual scheduler pods
oc get pods -n openshift-kube-scheduler
```

### Manual Verification

After making changes, you can verify manually:

```bash
# 1. Check the config
oc get scheduler cluster -o jsonpath='{.spec.mastersSchedulable}'

# 2. Try scheduling a test pod with control plane toleration
cat <<EOF | oc apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-scheduler
spec:
  containers:
  - name: test
    image: registry.access.redhat.com/ubi8/ubi-minimal:latest
    command: ['sleep', '3600']
  tolerations:
  - key: node-role.kubernetes.io/master
    operator: Exists
    effect: NoSchedule
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule
EOF

# 3. Check where it gets scheduled
oc get pod test-scheduler -o wide

# 4. Clean up
oc delete pod test-scheduler
```

## Dependencies

These utilities require:
- OpenShift client libraries (`github.com/openshift/client-go`)
- Kubernetes client libraries
- Proper RBAC permissions to read/write cluster configuration

### Required RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: scheduler-config-reader
rules:
- apiGroups: ["config.openshift.io"]
  resources: ["schedulers"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
```

## Best Practices

1. **Monitor After Changes**: Always monitor cluster performance after enabling control plane scheduling
2. **Use Resource Limits**: When scheduling workloads on control plane nodes, use appropriate resource requests and limits
3. **Gradual Rollout**: Test with non-critical workloads first
4. **Documentation**: Document any changes to scheduler configuration for your team
5. **Automation**: Consider automating the verification of scheduler configuration in your CI/CD pipelines 