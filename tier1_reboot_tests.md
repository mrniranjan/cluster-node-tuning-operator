# Tier1 Reboot Tests Analysis

## Summary

**Yes, there are Tier1 tests that trigger reboots/MCP rolls.**

Found **2 Tier1 test contexts** that involve node reboots or MCP updates:

---

## 1. Node Reboot Test (cgroups.go)

### **test_id:64099** - "Activation file doesn't get deleted"

**Location:** `test/e2e/performanceprofile/functests/7_performance_kubelet_node/cgroups.go:164`

**Labels:** `tier-1`

**What it does:**
```go
// Explicitly triggers node reboot
chroot /rootfs systemctl reboot
```

**Validation:**
- Reboots the worker node
- Waits for node to go NotReady
- Waits for node to come back Ready
- Verifies OVS activation file (`/etc/crio/ovs-activation`) still exists after reboot

**Runtime:** ~20 minutes (reboot cycle: drain + reboot + ready)

**Purpose:** Ensures OVS dynamic pinning activation file survives node reboots (important for upgrade scenarios)

**Blast radius:** **P1** - If this fails, OVS dynamic pinning breaks after node reboot/upgrade

**Runs on 4-CPU VMs:** ✅ Yes (no CPU count dependency)

**Current lane:** ✅ Fast serial (tier-1, not excluded)

---

## 2. RPS Mask Update Test (updating_profile.go)

### **test_id:56006** - "Verify systemd unit file gets updated when reserved CPUs modified"

**Location:** `test/e2e/performanceprofile/functests/2_performance_update/updating_profile.go:1108`

**Labels:** `tier-1`, `rps-mask`

**What it does:**
```go
// Triggers MCP roll by updating profile
profile.Spec.CPU.Reserved = <new CPUs>
profile.Spec.CPU.Isolated = <new CPUs>
WaitForTuningUpdating()
WaitForTuningUpdated()
```

**Validation:**
- Enables RPS (network stack pinning)
- Updates reserved/isolated CPUs
- Waits for MCP roll to complete
- Verifies RPS mask updated to match new reserved CPUs

**Runtime:** ~18 minutes (1 MCP roll)

**Purpose:** Ensures RPS systemd unit files get regenerated when CPU topology changes

**Blast radius:** **P1** - If this fails, RPS mask doesn't update when admins change CPU allocation → network stack pinning broken

**Runs on 4-CPU VMs:** ❌ **No - SKIPS on ≤8 CPUs** (line 1123-1125)

**Current lane:** ✅ Fast serial (tier-1, not excluded, but skips)

---

## Impact Analysis

### Why These Are Tier1 (Not Tier2)

Both tests validate **critical integration points** between PAO and node-level services:

1. **test_id:64099 (Node Reboot):**
   - Critical for **upgrade scenarios** (nodes reboot during upgrades)
   - OVS dynamic pinning is a **P1 feature** (real-time network performance)
   - Failure means: Upgrades break OVS pinning → network latency regression

2. **test_id:56006 (RPS Mask):**
   - Critical for **CPU topology changes** (cluster lifecycle)
   - RPS (Receive Packet Steering) is a **P1 feature** (network performance)
   - Failure means: CPU changes don't update network stack → silent perf degradation

**Tier1 criteria met:**
- ✅ Component-level functional tests
- ✅ P1 features (network performance, upgrade scenarios)
- ✅ Integration between PAO and node services (OVS, RPS)
- ✅ Must pass 100% (but don't block releases like P0)

---

## Current Lane Behavior

### Fast Serial Lane (`pao-functests-updating-profile`)

**Filter:** `!(hypershift||ovs-dpdk)`

**Tier1 reboot tests:**
- ✅ **64099 runs** (~20 min) - Node reboot, OVS activation file
- ⚠️ **56006 skips** on 4-CPU VMs (needs >8 CPUs)

**Total Tier1 reboot cost in fast lane:** ~20 minutes (just 64099)

---

## Should We Move Tier1 Reboot Tests?

**No, keep them in the fast serial lane.**

### Rationale:

1. **Only 1 actually runs** on 4-CPU VMs (64099, ~20 min)
2. **Not ovs-dpdk specific** - general OVS/RPS features
3. **P1 regression guards** - important for upgrades and CPU changes
4. **Tier1 semantic** - component-level functional, should run on PRs
5. **Total cost is low** - 20 min out of 193 min (10% of lane)

---

## Comparison: Tier1 vs Tier2 vs ovs-dpdk

| Category | Labels | Reboot? | Fast Lane? | Why |
|---|---|---|---|---|
| **Tier1 reboot** | tier-1 | ✅ Yes (~20m) | ✅ Yes | P1 features, upgrades |
| **Tier2 nodeSelector** | tier-2, openshift | ✅ Yes (~40m) | ✅ Yes | Important but not P1 |
| **ovs-dpdk** | ovs-dpdk, tier-2, slow | ✅ Yes (~45m) | ❌ No | Telco opt-in, not P1 |

**Key difference:**
- **Tier1:** Critical integration, upgrade scenarios, network perf → **Keep in fast**
- **Tier2:** Important but infrequent (lifecycle ops) → **Keep in fast for now**
- **ovs-dpdk:** Telco-specific, non-default → **Moved to nightly**

---

## Recommendation

**No action needed for Tier1 reboot tests.**

They are correctly placed in the fast serial lane because:
1. Only 1 runs on 4-CPU VMs (the other skips)
2. Critical for upgrade scenarios (P1)
3. Cost is acceptable (20 min / 10% of lane)

**If you want to optimize further:**
- Move test_id:56006 to a "high-CPU lane" (but it already skips on 4-CPU)
- Move nodeSelector tests (28440/27484) to nightly (save 40 min)

But for Tier1 tests, the current placement is correct.

---

## Summary Table

| test_id | Suite | Label | Reboot | Time | 4-CPU? | Fast Lane? | Critical? |
|---|---|---|---|---|---|---|---|
| **64099** | 7_kubelet | tier-1 | ✅ Yes | ~20 min | ✅ Runs | ✅ Yes | P1 (upgrades) |
| **56006** | 2_update | tier-1 | ✅ Yes | ~18 min | ❌ Skips | ⚠️ Yes (skips) | P1 (RPS) |

**Total Tier1 reboot cost on 4-CPU CI:** 20 minutes (acceptable)
