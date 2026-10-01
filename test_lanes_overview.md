# PAO E2E Test Lanes Overview

This document describes the optimized test lane structure for Performance Addon Operator (PAO) E2E tests.

---

## Lane Structure

### 1. **Tier1 Lane** (PR CI) - `make pao-functests-updating-profile`

**Purpose:** Component-level functional test validation (Tier0 + Tier1)  
**Runtime:** ~161 minutes  
**Trigger:** PR CI runs  

**What it runs:**
- ✅ All Tier0 tests (smoke/sanity)
- ✅ All Tier1 tests (component functional)
- ✅ Release-critical tests (P0/P1)
- ✅ Core RT kernel tuning validation
- ✅ CPU isolation, hugepages, kubelet config
- ✅ **Critical ovsDpdk test** (test_id:89987 - basic CPU config)
- ✅ OVS dynamic pinning, Tier1 reboot test
- ❌ Excludes: Tier2 tests (integration-level)

**Label filter:**
```bash
--label-filter='(tier-0||tier-1) && !hypershift'
```

**Suites:**
- 0_config (13 min)
- 1_performance (Tier0/Tier1 tests)
- 2_performance_update (Tier1 tests, includes test_id:89987)
- 3_performance_status (Tier1 status checks)
- 6_mustgather_testing (Tier1)
- 7_performance_kubelet_node (Tier1)
- 9_reboot (skipped on 4-CPU VMs)
- 10_performance_ppc (Tier1)
- 11_mixedcpus (Tier1)
- 13_llc (Tier1)

**Exit criteria:** Must pass 100% for PR merge

---

### 2. **Tier2 Lane** (Integration) - `make pao-functests-tier2`

**Purpose:** Integration-level functional test validation  
**Runtime:** ~85 minutes  
**Trigger:** Optional/informational on PRs, or periodic runs  

**What it runs:**
- ✅ All Tier2 tests (integration-level)
- ✅ ovs-dpdk lifecycle tests (10 tests, excluding basic config which is Tier1)
- ✅ nodeSelector tests (MCP retargeting)
- ✅ SMT housekeeping edge cases
- ✅ Other integration scenarios
- ❌ Excludes: Release-critical tests (those run in Tier1 lane)

**Label filter:**
```bash
--label-filter='tier-2 && !hypershift && !release-critical'
```

**Suites:**
- 0_config (profile setup)
- 2_performance_update (Tier2 tests)
- 7_performance_kubelet_node (Tier2 tests)

**Tests included:**
| Category | Tests | Time | Description |
|---|---|---|---|
| ovs-dpdk lifecycle | 10 specs | ~74 min | Lifecycle, integration, cleanup scenarios |
| nodeSelector | 2 specs | ~40 min | MCP retargeting (test_id:28440, 27484) |
| SMT housekeeping | 2 specs | ~13 min | Single-HT allocation edge cases |
| Other Tier2 | ~10 specs | ~10 min | Various integration tests |

**NO OVERLAP:** Tier1=(tier-0||tier-1), Tier2=(tier-2). Mutually exclusive.

**Exit criteria:** Should pass 100%, but doesn't block PRs (optional: true)

**Version note:** For backports to 4.x, remove this target (most Tier2 tests don't exist).

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

**PR CI (required for merge - Tier1):**
```yaml
- name: e2e-gcp-pao-tier1
  commands: make pao-functests-updating-profile
  timeout: 4h
```

**Optional/Informational (Tier2 integration):**
```yaml
- name: e2e-gcp-pao-tier2
  commands: make pao-functests-tier2
  optional: true  # Runs but doesn't block PR merge
  timeout: 2h
  # NOTE: Remove this lane entirely when backporting to 4.x (most Tier2 tests don't exist)
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
| **Tier1** | ~161 min | ~52 | ✅ Yes | Component functional (Tier0+Tier1) |
| **Tier2** | ~85 min | ~25 | ❌ No (optional) | Integration-level (Tier2) |
| **Release-Critical** | ~90 min | 36 | ✅ Yes (releases) | Release gating (cross-tier P0/P1) |
| **Original** | ~238 min | 53 | N/A | Legacy (before optimization) |

**Zero Overlap:** Tier1 and Tier2 lanes are mutually exclusive (tier-based, no tests run in both).

**Tier semantic:**
- **Tier0:** Unit/smoke tests (minimal time, 100% automated)
- **Tier1:** Component-level functional (includes critical ovsDpdk test_id:89987)
- **Tier2:** Integration-level functional (lifecycle, multi-component scenarios)

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
