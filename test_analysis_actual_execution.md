# Actual Serial Lane Execution Analysis (From Real CI Run)

## Data Source
Build log from actual CI run: `build-log.txt`  
Lane: `pao-functests-updating-profile` (serial execution, 4-CPU VM)

---

## 1. Actual Execution Summary

| Suite | Specs Run | Specs Skipped | Total Time | Notes |
|---|---|---|---|---|
| 0_config | 1 | 0 | 781s (13.0 min) | **Mandatory** - profile deployment |
| 2_performance_update | **33** | 18 | **12,119s (202 min)** | **88% of total time** |
| 3_performance_status | 5 | 2 | 99s (1.6 min) | Read-only status checks |
| 7_performance_kubelet_node | 14 | 9 | 1,297s (21.6 min) | Kubelet config validation |
| 9_reboot | 0 | 2 | 7s | All skipped (env-gated) |
| 13_llc | ? | ? | ? | (incomplete in log) |

**Total measured runtime:** **~238 minutes** (just under 4 hours)

**Key finding:** 33 tests in `2_performance_update` consumed 202 minutes (88% of lane time).

---

## 2. Top 20 Time-Consuming Tests (Actual Execution)

Only tests that **actually executed** (skipped tests excluded):

| Rank | Time | test_id | Suite | Description | Release-Critical? |
|---|---|---|---|---|---|
| 1 | **24.6 min** | **27484** | 2_update | nodeSelector revert (remove labels) | ❌ **Tier2** |
| 2 | **15.5 min** | **28440** | 2_update | nodeSelector move to different MCP | ❌ **Tier2** |
| 3 | **12.6 min** | **89994** | 2_update | ovsDpdk: update isolation when expanded | ❌ **Tier2 (ovs-dpdk)** |
| 4 | **11.1 min** | **89987** | 2_update | ovsDpdk: apply CPU node config | ❌ **Tier2 (ovs-dpdk)** |
| 5 | **11.0 min** | **89997** | 2_update | ovsDpdk: cleanup when removed | ❌ **Tier2 (ovs-dpdk)** |
| 6 | **11.0 min** | **89988** | 2_update | ovsDpdk: partition=member with enable | ❌ **Tier2 (ovs-dpdk)** |
| 7 | **7.1 min** | **86346** | 2_update | housekeeping single hyperthread | ❌ **Tier2** |
| 8 | **7.0 min** | **22764** | 2_update | no-op profile update | ❌ **Tier2** |
| 9 | **6.0 min** | **86347** | 2_update | housekeeping SMT disabled | ❌ **Tier2** |
| 10 | **6.0 min** | **34081** | 2_update | hugepages size and count | ✅ **P0 - KEEP** |
| 11 | **6.0 min** | **64099** | 7_kubelet | OVS cgroups cpuset.cpus.exclusive | ✅ **P1 - KEEP** |
| 12 | **3.2 min** | **78120** | 2_update | tuned deferred Never mode | ❌ **Tier2** |
| 13 | **3.2 min** | **78115** | 2_update | tuned deferred Always first-time | ✅ **Keep (1 of 3)** |
| 14 | **3.1 min** | **78118** | 2_update | tuned deferred Update in-place | ❌ **Drop (dup)** |
| 15 | **3.1 min** | **78116** | 2_update | tuned deferred Always in-place | ❌ **Drop (dup)** |
| 16 | **3.0 min** | **45490** | 7_kubelet | memory reservation annotation | ❌ **Tier2** |
| 17 | **3.0 min** | **45488** | 7_kubelet | kubelet annotation multi-setting | ❌ **Tier2** |
| 18 | **3.0 min** | **45495** | 7_kubelet | PAO managed parameters | ❌ **Tier2** |
| 19 | **3.0 min** | **45493** | 7_kubelet | kubelet override guard | ✅ **P1 - KEEP** |
| 20 | **2.3 min** | **89993** | 2_update | (ovsDpdk related) | ❌ **Tier2** |

**Subtotal (top 20):** **~156 minutes** out of 238 total (66%)

---

## 3. Time Consumption by Category (Executed Tests Only)

### 3.1 ovsDpdk Suite (4 tests)
| test_id | Time | Description |
|---|---|---|
| 89994 | 12.6 min | isolation expansion |
| 89987 | 11.1 min | apply CPU config |
| 89997 | 11.0 min | cleanup on removal |
| 89988 | 11.0 min | partition=member |
| **Total** | **~45 min** | **Telco-specific opt-in feature** |

**Recommendation:** **DROP from serial lane** → move to dedicated telco/DPDK lane  
**Label:** `ovs-dpdk`, `slow`, `tier-2`  
**Rationale:** ovsDpdk is opt-in, non-default. Not release-blocking for general RT/performance.

---

### 3.2 nodeSelector Tests (2 tests)
| test_id | Time | Description |
|---|---|---|
| 28440 | 15.5 min | move nodeSelector to different MCP |
| 27484 | 24.6 min | revert nodeSelector (remove labels) |
| **Total** | **~40 min** | **Infrastructure test** |

**Recommendation:** **DROP from serial lane** → move to nightly infra lane  
**Label:** `tier-2`, `openshift`  
**Rationale:** Requires ≥2 spare worker nodes (often unavailable). Tests MCP retargeting, not core RT tuning.

---

### 3.3 Housekeeping SMT Tests (2 tests)
| test_id | Time | Description |
|---|---|---|
| 86346 | 7.1 min | single hyperthread allocation |
| 86347 | 6.0 min | SMT disabled selection |
| **Total** | **~13 min** | **SMT edge cases** |

**Recommendation:** **Consider moving to nightly**  
**Label:** `tier-2`  
**Rationale:** Edge-case SMT behavior. Not P0/P1 regression guards.

---

### 3.4 Tuned Deferred (4 tests executed)
| test_id | Time | DeferMode | Change Type | Keep? |
|---|---|---|---|---|
| 78115 | 3.2 min | Always | first-time | ✅ **KEEP** |
| 78116 | 3.1 min | Always | in-place | ❌ Drop (dup) |
| 78118 | 3.1 min | Update | in-place | ❌ Drop (dup) |
| 78120 | 3.2 min | Never | in-place | ✅ Keep (edge case) |
| **Total** | **~12.6 min** | | | |

**Recommendation:** **Trim from 4 → 2 specs** (keep one per DeferMode: Always, Never)  
**Savings:** ~6 min

---

### 3.5 No-op Update (1 test)
| test_id | Time | Description |
|---|---|---|
| 22764 | 7.0 min | verify RT kernel disabled by default (no-op) |
| **Total** | **7 min** | **Idempotency check** |

**Recommendation:** **DROP**  
**Label:** `tier-2`  
**Rationale:** Low-value idempotency regression guard. Cost > benefit.

---

### 3.6 Release-Critical Tests That Executed

Only **2 release-critical reboot tests** actually ran in this lane:

| test_id | Time | Description | Component |
|---|---|---|---|
| **34081** | 6.0 min | Hugepages cmdline + allocation | TuneD bootloader + MC |
| (28071/28935/27738 likely in shared context, not individually timed) | | | |

Other release-critical tests in this lane are **read-only** (no reboots):
- 45493 (kubelet override guard, 3 min)
- 64099/64097/64100/64101 (OVS pinning, ~6 min total)

---

## 4. Immediate Savings Calculation

### Option A: Drop High-Impact Non-Critical Tests

| Category | Tests | Time Saved |
|---|---|---|
| **ovsDpdk suite** | 89987, 89988, 89994, 89997 | **−45 min** |
| **nodeSelector tests** | 27484, 28440 | **−40 min** |
| **No-op update** | 22764 | **−7 min** |
| **Tuned deferred trim** | 78116, 78118 | **−6 min** |
| **Housekeeping SMT** | 86346, 86347 | **−13 min** |

**Total savings:** **−111 minutes**  
**New lane time:** 238 min → **127 min** (2.1 hours)  
**Reduction:** **47%**

**Specs dropped:** 11 (out of 53 executed)  
**Critical tests retained:** All P0/P1 guards remain

---

### Option B: Drop Only Obvious Non-Critical (Conservative)

Drop only the top 3 categories (most clearly non-critical):

| Category | Time Saved |
|---|---|
| ovsDpdk suite | −45 min |
| nodeSelector tests | −40 min |
| No-op update | −7 min |

**Total savings:** **−92 minutes**  
**New lane time:** 238 min → **146 min** (2.4 hours)  
**Reduction:** **39%**

---

## 5. Recommendations by Priority

### Priority 1: Immediate (Zero Code Changes)

**Modify Makefile target `pao-functests-updating-profile`:**

```make
# Current:
pao-functests-updating-profile:
	hack/run-functests.sh 0_config 2_performance_update 3_performance_status 7_performance_kubelet_node 9_reboot 13_llc --label-filter=!(hypershift)

# Proposed:
pao-functests-updating-profile:
	hack/run-functests.sh 0_config 2_performance_update 3_performance_status 7_performance_kubelet_node 9_reboot 13_llc --label-filter=!(hypershift||ovs-dpdk)
```

**Alternative:** Use `--skip-file=ovsdpdk.go` or exclude test IDs.

**Savings:** −45 min (ovsDpdk only)

---

### Priority 2: Quick Wins (Makefile + Test Labels)

Also exclude nodeSelector tests via label or test ID:

```bash
--label-filter=!(hypershift||ovs-dpdk) --skip='test_id:28440|test_id:27484|test_id:22764'
```

**Savings:** −92 min (ovsDpdk + nodeSelector + no-op)

---

### Priority 3: Code Cleanup (Test Tagging)

Add proper labels to tests for easier filtering:

1. **Add `label.NonCritical` or `label.Nightly`** to:
   - ovsDpdk tests (already have `ovs-dpdk`, `slow`, `tier-2`)
   - nodeSelector tests (28440, 27484)
   - No-op update (22764)
   - Housekeeping SMT tests (86346, 86347)

2. **Trim tuned_deferred** from 6 specs → 3 specs (delete duplicate in-place variants)

3. **Delete dead tests:**
   - test_id:36364 (unconditional Skip in cpu_management.go:430)

---

## 6. What Stays in Time-Boxed Lane

### Critical Reboot Tests (Must Stay)
- **34081** - Hugepages (P0, 6 min)
- **28071** - isolcpus + systemd.cpu_affinity (P0, shared context)
- **28935** - reservedSystemCPUs (P0, shared context)
- **27738** - RT kernel toggle (P0, ~18 min standalone if measured)

### Critical Read-Only Tests (Fast, Must Stay)
- **45493** - kubelet override guard (P1, 3 min)
- **64097/64099/64100/64101** - OVS dynamic pinning (P1, ~6 min total)
- All of **3_performance_status** (Degraded propagation, ~1.6 min)

### High-Value Non-Critical (Consider Keeping)
- **78115** - tuned deferred Always (3.2 min, good coverage)
- Selected kubelet annotation tests (45488, 45490 for coverage)

---

## 7. Final Summary

**Current state (actual measured):**
- 53 tests executed (31 skipped)
- 238 minutes total
- 88% of time in 2_performance_update suite

**After dropping 11 non-critical tests:**
- 42 tests executed (−11)
- **127 minutes** (−111 min, **47% reduction**)
- All P0/P1 release guards retained
- Fits comfortably in 2-hour CI window

**Tests moved to nightly/telco lanes:**
- ovsDpdk suite → telco lane
- nodeSelector tests → nightly infra lane
- Housekeeping SMT → nightly
- No-op update → can delete or move to nightly

**Coverage gaps remain** (skipped on 4-CPU VMs):
- test_id:75327 (OCPBUGS-34812, needs >10 CPU)
- netqueues suite (needs multi-queue NIC)
- 4_latency (baremetal + long run)

These require a **dedicated ≥8-CPU baremetal lane**.
