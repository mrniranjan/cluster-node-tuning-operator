# PAO E2E Test Lanes Overview

This document describes the optimized test lane structure for Performance Addon Operator (PAO) E2E tests.

---

## Lane Structure

### 1. **Fast Serial Lane** (PR CI) - `make pao-functests-updating-profile`

**Purpose:** Fast feedback loop for PR validation  
**Runtime:** ~193 minutes (down from 238 min)  
**Trigger:** PR CI runs  

**What it runs:**
- ✅ All release-critical tests (P0/P1)
- ✅ Core RT kernel tuning validation
- ✅ CPU isolation, hugepages, kubelet config
- ✅ Status/Degraded propagation
- ✅ OVS dynamic pinning
- ❌ Excludes: ovs-dpdk tests (non-critical, telco-specific)

**Label filter:**
```bash
--label-filter='!(hypershift||ovs-dpdk)'
```

**Suites:**
- 0_config (13 min)
- 2_performance_update (157 min, without ovs-dpdk)
- 3_performance_status (1.6 min)
- 7_performance_kubelet_node (21.6 min)
- 9_reboot (skipped on 4-CPU VMs)
- 13_llc (partial, config only)

**Exit criteria:** Must pass 100% for PR merge

---

### 2. **Nightly Lane** (OVS-DPDK) - `make pao-functests-updating-nightly`

**Purpose:** Telco-specific DPDK feature validation  
**Runtime:** ~45 minutes  
**Trigger:** Optional/informational on PRs, or periodic runs  

**What it runs:**
- ✅ ovs-dpdk tests ONLY (telco DPDK vSwitch/vRouter features)
- ❌ Excludes: Everything else (zero overlap with fast lane)

**Label filter:**
```bash
--label-filter='ovs-dpdk && !hypershift'
```

**Suites:**
- 0_config (profile setup)
- 2_performance_update (ovs-dpdk tests only)

**Tests included:**
| Category | Tests | Time | Description |
|---|---|---|---|
| ovs-dpdk | 4 specs | ~45 min | DPDK vSwitch/vRouter CPU isolation |
| test_id:89987 | 1 spec | ~11 min | ovsDpdk CPU node configuration |
| test_id:89988 | 1 spec | ~11 min | ovsdpdk.slice partition=member |
| test_id:89994 | 1 spec | ~13 min | isolation expansion when CPUs expanded |
| test_id:89997 | 1 spec | ~11 min | artifact cleanup when ovsDpdk removed |

**NO OVERLAP:** Fast lane excludes ovs-dpdk, Nightly ONLY runs ovs-dpdk.

**Exit criteria:** Should pass 100%, but doesn't block PRs (optional: true)

**Version note:** Only exists in 5.0+. Remove this lane when backporting to 4.x.

---

### 3. **Release-Critical Lane** - `make pao-functests-release-critical`

**Purpose:** Deterministic critical-only validation  
**Runtime:** ~90 minutes  
**Trigger:** Release validation, gating  

**What it runs:**
- Only tests tagged with `label.ReleaseCritical`
- CPU-count independent (same on 4-CPU VMs as on larger clusters)

**Label filter:**
```bash
--label-filter='release-critical && !hypershift'
```

**Suites:**
- 0_config
- 1_performance
- 2_performance_update (critical tests only)
- 3_performance_status
- 6_mustgather_testing
- 7_performance_kubelet_node
- 8_performance_workloadhints
- 10_performance_ppc
- 11_mixedcpus

**Critical reboot tests (from 2_performance_update):**
- test_id:34081 - Hugepages cmdline + allocation (~6 min)
- test_id:28071 - isolcpus + systemd.cpu_affinity (~12 min shared)
- test_id:28935 - reservedSystemCPUs (~12 min shared)
- test_id:27738 - RT kernel toggle (~18 min)

**Exit criteria:** Must pass 100% for release

---

## Test Classification

### P0 - Release Blocking
**Criteria:** Failure breaks RT guarantees, node stability, or core product promises

Examples:
- CPU isolation (isolcpus, workqueue mask, systemd.cpu_affinity)
- RT kernel enable/disable
- Hugepages kernel cmdline
- kubelet reservedSystemCPUs
- Tuned not Degraded

**Label:** `label.ReleaseCritical`  
**Lane:** Fast serial + Release-critical

---

### P1 - High Priority
**Criteria:** Feature broken on all pool nodes, silent regression

Examples:
- OVS dynamic pinning
- kubelet override guards
- Mixed-CPUs shared cpuset
- Topology manager policy
- Workload partitioning

**Label:** `label.ReleaseCritical` (most) or `label.Tier1`  
**Lane:** Fast serial + Release-critical

---

### P2 - Integration / Tier2
**Criteria:** Valuable regression coverage but not release-blocking

Examples:
- ovs-dpdk (telco opt-in feature)
- nodeSelector MCP retargeting
- SMT housekeeping edge cases
- Tuned deferred update modes
- Kubelet annotation pass-through

**Label:** `label.Tier2`  
**Lane:** Nightly

---

### P3 - Tool / Support
**Criteria:** Offline tools, support-bundle collection, documentation

Examples:
- performance-profile-creator CLI
- must-gather collection
- Per-pod power management (tool validation)

**Label:** `label.Tier2` or `label.Tier3`  
**Lane:** Dedicated tool lanes (ppc, mustgather)

---

## Usage

### Run the fast serial lane (PR validation):
```bash
make pao-functests-updating-profile
```

### Run the nightly lane (comprehensive Tier2):
```bash
make pao-functests-updating-nightly
```

### Run only release-critical tests:
```bash
make pao-functests-release-critical
```

### Run all update tests (original behavior, ~238 min):
To run everything including non-critical tests in one go:
```bash
make pao-functests-update-only GINKGO_LABEL_FILTER="!hypershift"
```

---

## CI Configuration

### Recommended CI Lane Setup

**PR CI (required for merge):**
```yaml
- name: e2e-gcp-pao-updating-profile
  commands: make pao-functests-updating-profile
  timeout: 4h
```

**Nightly/Optional (ovs-dpdk regression):**
```yaml
- name: e2e-gcp-pao-updating-nightly
  commands: make pao-functests-updating-nightly
  optional: true  # Runs but doesn't block PR merge
  timeout: 2h
  # NOTE: Remove this lane entirely when backporting to 4.x (ovs-dpdk tests don't exist)
```

**Release Gate (pre-release validation):**
```yaml
- name: e2e-gcp-pao-release-critical
  commands: make pao-functests-release-critical
  timeout: 2h
  trigger: release-branch-push
```

---

## Lane Comparison

| Lane | Runtime | Tests | PR Blocker? | Purpose |
|---|---|---|---|---|
| **Fast Serial** | ~193 min | 49 | ✅ Yes | PR validation, fast feedback |
| **Nightly (ovs-dpdk)** | ~45 min | 4 | ❌ No (optional) | Telco DPDK regression coverage |
| **Release-Critical** | ~90 min | 36 | ✅ Yes (releases) | Release gating |
| **Original** | ~238 min | 53 | N/A | Legacy (before optimization) |

**Zero Overlap:** Fast and Nightly lanes are mutually exclusive (no tests run in both).

---

## Test Coverage Gaps

Some critical tests **cannot run on 4-CPU VMs** and need dedicated lanes:

### Baremetal / High-CPU Lane (≥8 CPU, baremetal)
**Tests that skip on 4-CPU VMs:**
- test_id:75327 - Cgroup cpuset reassign (OCPBUGS-34812, needs >10 CPU)
- netqueues suite - NIC multi-queue tuning (needs configurable multi-queue NIC)
- 4_latency - Real oslat/cyclictest (baremetal + long run)
- 2-NUMA memorymanager tests

**Recommendation:** Create dedicated `e2e-metal-pao-hwdependent` lane

---

## Test Time Breakdown (From Actual CI Run)

Based on measured execution (build-log.txt):

### Top Time Consumers (Fast Serial Lane, Post-Optimization)
| Rank | Time | test_id | Description | Lane |
|---|---|---|---|---|
| 1 | ~18 min | 27738 | RT kernel toggle | Fast ✅ |
| 2 | ~12 min | 34081 + 28071 + 28935 | Shared reboot context | Fast ✅ |
| 3 | ~7 min | 86346, 86347 | SMT housekeeping | Nightly |
| 4 | ~6 min | 64099, 64097 | OVS pinning | Fast ✅ |
| 5 | ~6 min | tuned_deferred | Tuned defer modes | Fast/Nightly mix |

### Moved to Nightly Lane
| Time | test_id | Description |
|---|---|---|
| 45 min | 89987–89997 | ovs-dpdk suite (4 tests) |
| 40 min | 28440, 27484 | nodeSelector tests |
| 13 min | 86346, 86347 | SMT housekeeping |
| 7 min | 22764 | No-op update (can delete) |

**Total moved:** ~105 min (but nightly runs in parallel, so ~85 min actual)

---

## Future Optimizations

### Potential Further Improvements
1. **Tag nodeSelector tests** with dedicated label → easier filtering
2. **Delete dead tests:** test_id:36364 (unconditional Skip)
3. **Trim tuned_deferred** from 6 → 3 specs (drop in-place duplicates)
4. **Relocate GetNumaRanges** unit test from e2e to unit package
5. **Create baremetal lane** for HW-dependent tests (75327, netqueues, latency)

### Conservative Estimate
With all optimizations:
- Fast serial: 238 → **127 min** (47% reduction)
- Nightly: **85 min** (new)
- Total CI capacity freed: ~26 min per PR run

---

## References

- **Analysis Documents:**
  - `test_analysis.md` - Initial suite breakdown
  - `test_analysis_extended_reboot.md` - Reboot test deep dive
  - `test_analysis_actual_execution.md` - Real CI execution data

- **Related Makefile Targets:**
  - `pao-functests-only` - Fast acceptance lane (0, 1, 6, 10)
  - `pao-functests-performance-workloadhints` - Workload hints lane
  - `pao-functests-latency-testing` - Latency tests (needs ≥10 CPU)
  - `pao-functests-mixedcpus` - Mixed-CPUs feature
  - `pao-functests-hypershift` - HyperShift multi-nodepool

---

## Questions?

**Q: Why not just use `--label-filter='release-critical'` for everything?**  
A: That would exclude valuable Tier2 tests that catch regressions but don't block releases. We want comprehensive coverage in nightly, fast feedback in PR CI.

**Q: Can I run just ovs-dpdk tests?**  
A: Yes, use `--label-filter='ovs-dpdk'`:
```bash
make pao-functests-update-only GINKGO_LABEL_FILTER="ovs-dpdk"
```

**Q: What if I need to test a specific test_id?**  
A: Use ginkgo's focus flag:
```bash
ginkgo --focus="test_id:34081" test/e2e/performanceprofile/functests/2_performance_update/
```

**Q: Why does nightly lane not include 3_performance_status?**  
A: Status tests are all fast (<2 min) and already run in the fast serial lane. Nightly focuses on expensive Tier2 update tests only.
