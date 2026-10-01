# NodeSelector Tests Impact Analysis

## Test IDs: 28440 and 27484

These tests validate the **PerformanceProfile nodeSelector retargeting** feature - the ability to move performance tuning from one Machine Config Pool (MCP) to another by changing node labels.

---

## What These Tests Do

### Test 28440: "Verifies that nodeSelector can be updated in performance profile"

**Setup (BeforeEach):**
1. Takes a spare worker node (not currently managed by the performance profile)
2. Creates a NEW Machine Config Pool (MCP) called `worker-test`
3. Updates the PerformanceProfile's `nodeSelector` to target the new MCP
4. Waits for MCP to roll the performance configuration to the new node

**Validation:**
- Verifies the new node received the performance tuning:
  - `kubeletConfig.TopologyManagerPolicy` is set (kubelet configuration applied)
  - Kernel cmdline contains `tuned.non_isolcpus` (TuneD boot params applied)

**Duration:** ~16 minutes (1 MCP roll to apply config to new node)

---

### Test 27484: "Verifies that node is reverted to plain worker when labels are removed"

**Setup (runs after 28440):**
1. Removes the performance labels from the node that was configured in 28440
2. Node should revert from "performance worker" back to "plain worker"
3. Waits for MCP to roll the reversion

**Validation:**
- Verifies the node reverted to plain worker state:
  - RT kernel is NOT present (`HasPreemptRTKernel` returns error)
  - Kernel cmdline does NOT contain `tuned.non_isolcpus`
  - `kubeletConfig.ReservedSystemCPUs` is empty/unset

**Duration:** ~24 minutes (1 MCP roll to revert + cleanup)

---

## What Failure Indicates

### If Test 28440 Fails:

**Problem:** PerformanceProfile cannot retarget to a different MCP

**Root Causes:**
1. **Controller reconcile logic broken** - Performance Profile controller not watching/reacting to `nodeSelector` changes
2. **MachineConfig generation broken** - Controller generates MachineConfig but doesn't apply it to new MCP
3. **KubeletConfig generation broken** - kubelet settings not rendered for new nodeSelector
4. **TuneD template broken** - TuneD profile not applied to new nodes
5. **MCP selector logic broken** - Controller can't determine which MCP to target

**Code Components Implicated:**
- `pkg/performanceprofile/controller/performanceprofile/performanceprofile_controller.go` - reconcile loop
- `pkg/performanceprofile/controller/performanceprofile/components/handler/handler.go` - Apply() function
- `pkg/performanceprofile/controller/performanceprofile/components/manifestset/` - component generation

---

### If Test 27484 Fails:

**Problem:** Nodes don't properly revert when removed from performance pool

**Root Causes:**
1. **Orphaned MachineConfigs** - Old performance MachineConfigs not cleaned up when nodeSelector changes
2. **MCP cleanup broken** - MCO (Machine Config Operator) not reverting nodes when they leave a pool
3. **TuneD cleanup broken** - TuneD profiles persist after labels removed
4. **Kubelet config cleanup broken** - Performance kubelet settings persist
5. **Finalizer logic broken** - PerformanceProfile finalizers not cleaning up properly

**Code Components Implicated:**
- `pkg/performanceprofile/controller/performanceprofile/` - finalizer logic
- MCO interaction (external dependency)
- Node labeling/MCP membership reconciliation

---

## Real-World Impact if These Features Break

### Impact: Infrastructure/Fleet Management

**Severity:** **P2 - Integration-level** (not release-blocking, but important)

### Scenario 1: Cluster Expansion (Test 28440 broken)

**Customer workflow:**
```
1. Customer deploys initial cluster with 3 worker nodes as RT workers
2. Later, they add 10 new worker nodes for RT workloads
3. They update PerformanceProfile nodeSelector to include new nodes
```

**If broken:**
- ❌ New nodes don't get performance tuning
- ❌ RT workloads scheduled to new nodes fail (no CPU isolation, no RT kernel)
- ❌ Silent data plane failure - pods run but miss latency SLAs
- ⚠️ **Workaround:** Delete and recreate PerformanceProfile (disruptive)
- ⚠️ **Workaround:** Manually label nodes before cluster expansion (operational burden)

**Affected customers:**
- Telco CNFs scaling RT worker pools
- Edge deployments adding compute capacity
- HyperShift multi-nodepool environments

---

### Scenario 2: Pool Consolidation (Test 27484 broken)

**Customer workflow:**
```
1. Customer has dedicated "worker-rt" pool with 5 nodes
2. Decides to consolidate: remove RT from 2 nodes, repurpose as plain workers
3. They remove performance labels from 2 nodes
```

**If broken:**
- ❌ Nodes keep RT kernel + performance tuning even after label removal
- ❌ Generic workloads see unexpected behavior (reserved CPUs, isolated cores)
- ❌ Capacity planning broken (node reports wrong allocatable resources)
- ❌ **No workaround** - node must be drained + reimaged to clean state

**Affected customers:**
- Anyone decommissioning RT workers
- Clusters with seasonal capacity (add RT nodes for peak, remove after)
- Lab/dev environments repurposing nodes

---

### Scenario 3: MCP Renaming/Restructuring (Both tests broken)

**Customer workflow:**
```
1. Customer has "worker-cnf" MCP with performance tuning
2. Wants to split into "worker-cnf-du" and "worker-cnf-cu" pools
3. They create new MCPs and update PerformanceProfile nodeSelector
```

**If broken:**
- ❌ New MCPs don't get performance config (28440 broken)
- ❌ Old MCP nodes stuck with stale config (27484 broken)
- ❌ Cluster in split-brain state: some nodes RT, some not, unclear which is which
- ⚠️ **Recovery:** Cluster reinstall (customer data loss if not backed up)

**Affected customers:**
- Multi-tenant telco clusters (per-tenant MCPs)
- Gitops-managed fleet migrations
- Disaster recovery scenarios

---

## Why These Tests Are NOT Release-Critical

Despite the severe impact when broken, these tests are **Tier2** (not release-critical) because:

### 1. **Infrequent Operation**
- NodeSelector changes happen during **cluster lifecycle events** (expansion, consolidation)
- NOT part of normal steady-state operation
- Most clusters: set nodeSelector once during initial deployment, never change

### 2. **Observable Before Production**
- Breaks during staged rollout (dev → stage → prod)
- Admin can verify new nodes received config before moving traffic
- Not a silent runtime failure

### 3. **Workarounds Exist**
- Test 28440 broken: Label nodes before profile update, or recreate profile
- Test 27484 broken: Drain + reimage nodes (disruptive but viable)

### 4. **Infrastructure Test, Not Data Plane**
- Doesn't break RT guarantees on **existing** running nodes
- Doesn't cause latency spikes or packet loss on active workloads
- Impact is on **change management**, not **runtime behavior**

### 5. **External Dependency (MCO)**
- Heavy reliance on Machine Config Operator behavior
- MCO itself can have bugs in MCP switching logic
- PAO just generates the MachineConfigs; MCO applies them

---

## Where the Impact Occurs

### Component Layer:

| Layer | Component | Impact if Broken |
|---|---|---|
| **API/CRD** | PerformanceProfile.Spec.NodeSelector | Accepted but ignored |
| **Controller** | performanceprofile_controller.go reconcile | Doesn't react to nodeSelector changes |
| **Render** | components/machineconfig.go | Generates MC for wrong MCP |
| **Render** | components/kubeletconfig.go | Doesn't update KC nodeSelector |
| **Render** | components/tuned.go | TuneD recommend doesn't match new nodes |
| **MCO** | MachineConfigDaemon (external) | Doesn't apply/revert MC on MCP changes |
| **Node** | kubelet, tuned daemon | Stuck with stale config |

---

### Blast Radius:

**Scope:** Nodes transitioning between MCPs  
**Severity:** Infrastructure-level (not data-plane RT failure)  
**Customers affected:** Only those actively changing nodeSelector  
**Data plane impact:** New nodes don't get RT tuning (fail-open, not fail-closed)

---

## Test Execution Cost Analysis

### Why These Tests Are Expensive

**Test 28440: 16 minutes**
1. Create new MCP: ~30 sec
2. Update PerformanceProfile: ~10 sec
3. Wait for MCP "Updating" condition: ~30 sec
4. **MCP roll (apply MC to node):** ~14 min ← **the expensive part**
5. Validate node config: ~1 min

**Test 27484: 24 minutes**
1. Remove node labels: ~10 sec
2. Wait for MCP "Updating" condition: ~30 sec
3. **MCP roll (revert MC on node):** ~18 min ← **the expensive part**
4. Validate node reverted: ~1 min
5. **Cleanup + restore old config:** ~5 min ← **also expensive**

**Why MCP rolls are slow:**
- Node drain (graceful pod eviction): ~2-5 min
- Reboot (kernel change or systemd unit updates): ~3-5 min
- MCO daemon apply + verify: ~2-3 min
- Node ready + scheduling enabled: ~1-2 min
- Safety delays (MCO doesn't rush): ~3-5 min

---

## Why We Moved These to Nightly Lane

### Decision Criteria:

✅ **Value:** Important regression coverage for fleet management  
❌ **Criticality:** Not release-blocking (infrastructure, not data plane)  
❌ **Frequency:** Infrequent customer operation  
✅ **Cost:** 40 minutes (17% of total lane time)  
✅ **CI resource:** Requires ≥2 spare worker nodes (often unavailable on small CI clusters)  

**Verdict:** Run in nightly for full coverage, skip in fast PR lane for velocity

---

## Dependencies

These tests validate **PAO + MCO + TuneD** integration, specifically:

1. **PAO Controller:**
   - Watches PerformanceProfile.Spec.NodeSelector changes
   - Updates MachineConfig/KubeletConfig/Tuned with new selectors
   
2. **MCO (Machine Config Operator):**
   - Detects node moved between MCPs (label change)
   - Applies new MachineConfig to node
   - Reverts old MachineConfig when node leaves pool
   - Orchestrates drain + reboot + verify

3. **TuneD Operator:**
   - Updates TuneD recommend.conf to match new nodeSelector
   - Node's tuned daemon picks new profile
   - Old profile deactivated on label removal

4. **Kubelet:**
   - Picks up new KubeletConfig from MCO
   - Applies reservedSystemCPUs, topologyManagerPolicy, etc.
   - Reverts to default when KubeletConfig removed

---

## Failure Triage Guide

If these tests fail, triage in this order:

### 1. Check MCO Health
```bash
oc get mcp -A  # Are MCPs stuck Updating?
oc get mc | grep performance  # Are MachineConfigs generated?
oc logs -n openshift-machine-config-operator deployment/machine-config-controller
```

**Common MCO issues:**
- MCP stuck draining nodes
- MCO controller crashlooping
- Render errors in MachineConfig

---

### 2. Check PerformanceProfile Controller Logs
```bash
oc logs -n openshift-cluster-node-tuning-operator deployment/cluster-node-tuning-operator | grep performance
```

**Look for:**
- "reconcile PerformanceProfile" messages
- "Updating MachineConfig for MCP" logs
- Errors generating components

---

### 3. Check Node State
```bash
oc get nodes -l <new-label>  # Did node get the new label?
oc debug node/<node> -- chroot /host /bin/bash -c "cat /proc/cmdline"  # Check tuned.non_isolcpus
oc debug node/<node> -- chroot /host /bin/bash -c "cat /etc/kubernetes/kubelet.conf"  # Check reservedSystemCPUs
```

---

### 4. Check Generated Resources
```bash
oc get performanceprofile -o yaml  # Check status conditions
oc get mc -o yaml | grep -A 20 "name: 99-.*-performance"  # Check MachineConfig selectors
oc get kubeletconfig -o yaml  # Check nodeSelector matches
oc get tuned -n openshift-cluster-node-tuning-operator -o yaml  # Check recommend.conf
```

---

## Summary

**What:** Tests validate PerformanceProfile nodeSelector retargeting (moving RT tuning between MCPs)

**Impact if broken:**
- **Cluster expansion:** New nodes don't get RT tuning → workload failures
- **Pool consolidation:** Nodes stuck with RT config → capacity planning breaks
- **MCP restructuring:** Cluster split-brain → recovery requires reinstall

**Blast radius:** Infrastructure/fleet management layer (not data-plane RT failures)

**Why Tier2 (not release-critical):**
- Infrequent operation (lifecycle events, not steady-state)
- Observable before prod (staged rollouts catch it)
- Workarounds exist (recreate profile, reimage nodes)
- External MCO dependency

**Why expensive:** Each test triggers full MCP roll (drain + reboot + verify) = 16-24 min per test

**Where it runs:** Nightly lane (comprehensive regression coverage without blocking PRs)
