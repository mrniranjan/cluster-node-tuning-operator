# Tier-Based Lane Proposal

## Current Problem

The current lane split is **feature-based** (ovs-dpdk vs non-ovs-dpdk), not **tier-based**:
- Fast lane: Everything except ovs-dpdk
- Nightly lane: Only ovs-dpdk

This doesn't follow the Tier semantic properly.

---

## Proposed Solution: Tier-Based Lanes

### **Lane 1: Tier0/Tier1 Lane** (Fast, PR-blocking)
**Runtime:** ~150 min  
**Filter:** `(tier-0 || tier-1) && !hypershift`  
**Purpose:** Component-level functional tests, PR validation

**What runs:**
- ✅ All Tier0 tests (unit/smoke)
- ✅ All Tier1 tests (component functional)
- ✅ Includes critical ovs-dpdk tests (promoted to Tier1)
- ✅ Includes Tier1 reboot tests (64099, 56006)

---

### **Lane 2: Tier2 Lane** (Nightly/Optional, non-blocking)
**Runtime:** ~85 min  
**Filter:** `tier-2 && !hypershift && !release-critical`  
**Purpose:** Integration-level regression coverage

**What runs:**
- ✅ All Tier2 tests
- ✅ Includes non-critical ovs-dpdk tests (remain Tier2)
- ✅ Includes nodeSelector tests (28440, 27484)
- ✅ Includes SMT housekeeping
- ❌ Excludes release-critical tests (already in fast lane)

---

### **Lane 3: Release-Critical Lane** (Release gating)
**Runtime:** ~90 min  
**Filter:** `release-critical && !hypershift`  
**Purpose:** Deterministic release validation

**What runs:**
- ✅ Only tests tagged release-critical (cross-tier)
- ✅ P0 tests regardless of tier

---

## Step 1: Classify ovs-dpdk Tests

Need to decide which ovs-dpdk tests are **Tier1** (component functional) vs **Tier2** (integration).

### **Criteria for Tier1:**
- Tests basic ovsDpdk functionality (CPU isolation, kernel params)
- Single-feature validation (not multi-component integration)
- Telco customers need this to work (even if opt-in)
- Fast enough for PR feedback (~10-15 min)

### **Criteria for Tier2:**
- Tests complex scenarios (pod lifecycle, updates, cleanup)
- Multi-component integration (ovs + kubelet + tuned + cgroups)
- Expensive (>15 min)
- Nice-to-have regression coverage

---

## ovs-dpdk Test Analysis

Based on build log and code inspection:

| test_id | Description | Suite | Time | Tier Recommendation | Rationale |
|---|---|---|---|---|---|
| **89987** | apply ovsDpdk CPU node configuration | ovsdpdk.go:138 | ~11 min | **Tier1** | Core functionality: validates isolcpus, nohz_full, rcu_nocbs, kubelet config. Basic sanity check. |
| **89988** | partition=member with cpu-load-balancing | ovsdpdk.go:371 | ~11 min | **Tier2** | Integration scenario: cgroup partition mode + load balancing. Nice-to-have. |
| **89994** | isolation expansion when CPUs expanded | ovsdpdk.go (Ordered #2) | ~13 min | **Tier2** | Update scenario: changing CPU topology. Lifecycle test, not basic functionality. |
| **89997** | cleanup when ovsDpdk removed | ovsdpdk.go (Ordered #2) | ~11 min | **Tier2** | Cleanup scenario: removal/reversal. Integration test, not core func. |

**Recommendation:**
- **Promote to Tier1:** test_id:89987 only (~11 min)
- **Keep as Tier2:** test_id:89988, 89994, 89997 (~35 min total)

---

## Proposed Lane Composition

### **Tier1 Lane (Fast, PR-blocking)**

**Filter:** `(tier-0 || tier-1) && !hypershift`  
**Runtime:** ~161 min (current 193 - 32 from removed tier-2 tests)

**Tests added:**
- ✅ test_id:89987 - ovsDpdk apply config (~11 min)

**Tests removed:**
- ❌ nodeSelector tests (28440, 27484) → moved to Tier2 lane
- ❌ SMT housekeeping (86346, 86347) → moved to Tier2 lane
- ❌ Other Tier2 non-critical tests

**Tests staying:**
- ✅ All release-critical tests (P0/P1)
- ✅ Tier1 reboot test (64099 - ~20 min)
- ✅ Tier1 RPS test (56006 - skips on 4-CPU)
- ✅ All Tier0 tests

---

### **Tier2 Lane (Nightly/Optional)**

**Filter:** `tier-2 && !hypershift && !release-critical`  
**Runtime:** ~85 min

**Tests included:**
- ✅ ovs-dpdk tests 89988, 89994, 89997 (~35 min)
- ✅ nodeSelector tests 28440, 27484 (~40 min)
- ✅ SMT housekeeping 86346, 86347 (~13 min)
- ✅ Other Tier2 integration tests

---

## Implementation Plan

### **Step 1: Re-label ovs-dpdk Tests**

```go
// test/e2e/performanceprofile/functests/2_performance_update/ovsdpdk.go

// Promote to Tier1 - basic functionality test
It("should apply ovsDpdk CPU node configuration", Label(string(label.Tier1)), func() {
    // test_id:89987 - validates kernel params, kubelet config
})

// Keep as Tier2 - integration/lifecycle tests
It("should configure ovsdpdk.slice with partition=member", Label(string(label.Tier2)), func() {
    // test_id:89988
})

It("should update isolation when ovsDpdk CPUs are expanded", Label(string(label.Tier2)), func() {
    // test_id:89994
})

It("should clean up all ovsDpdk artifacts when ovsDpdk is removed", Label(string(label.Tier2)), func() {
    // test_id:89997
})
```

---

### **Step 2: Update Makefile**

```make
# Tier1 Lane (Fast, PR-blocking)
.PHONY: pao-functests-update-only
pao-functests-update-only: $(BINDATA)
	@echo "Cluster Version"
	hack/show-cluster-version.sh
	hack/run-test.sh -t "test/e2e/performanceprofile/functests/0_config test/e2e/performanceprofile/functests/1_performance test/e2e/performanceprofile/functests/2_performance_update test/e2e/performanceprofile/functests/3_performance_status test/e2e/performanceprofile/functests/6_mustgather_testing test/e2e/performanceprofile/functests/7_performance_kubelet_node test/e2e/performanceprofile/functests/9_reboot test/e2e/performanceprofile/functests/10_performance_ppc test/e2e/performanceprofile/functests/11_mixedcpus test/e2e/performanceprofile/functests/13_llc" -p "-v -r --label-filter='(tier-0||tier-1) && !hypershift' --fail-fast --flake-attempts=2 --timeout=4h --junit-report=report.xml" -m "Running Tier0/Tier1 Functional Tests"

# Tier2 Lane (Nightly/Optional)
.PHONY: pao-functests-tier2-only
pao-functests-tier2-only: $(BINDATA)
	@echo "Cluster Version"
	hack/show-cluster-version.sh
	hack/run-test.sh -t "test/e2e/performanceprofile/functests/0_config test/e2e/performanceprofile/functests/2_performance_update test/e2e/performanceprofile/functests/7_performance_kubelet_node" -p "-v -r --label-filter='tier-2 && !hypershift && !release-critical' --flake-attempts=2 --timeout=2h --junit-report=report-tier2.xml" -m "Running Tier2 Integration Tests"
```

---

## Benefits

### ✅ **Semantic Clarity**
- Tier0/Tier1 = Component functional (fast feedback)
- Tier2 = Integration (comprehensive coverage)
- Aligns with standard test tier definitions

### ✅ **Proper Coverage**
- Critical ovs-dpdk functionality (89987) runs on every PR
- Integration ovs-dpdk tests (lifecycle, updates) run in Tier2

### ✅ **Flexibility**
- Easy to promote/demote tests by changing labels
- Clear criteria for Tier1 vs Tier2

### ✅ **Backward Compatible**
- Tier labels already exist in codebase
- Tier1 lane works in 4.x (test_id:89987 doesn't exist, but other Tier1 tests do)

---

## Migration Path

### **For 5.0+ (master, release-5.0):**

1. Re-label test_id:89987 as Tier1
2. Update Makefile to use tier-based filters
3. Update CI config to run Tier1 (blocking) + Tier2 (optional)

### **For 4.x backports:**

1. Only backport Tier1 lane optimization
2. Tier2 lane not needed (no ovs-dpdk tests in 4.x)
3. Tier1 filter naturally excludes non-existent tests

---

## Comparison: Current vs Proposed

| Aspect | Current (Feature-Based) | Proposed (Tier-Based) |
|---|---|---|---|
| **Fast Lane Filter** | `!(hypershift\|\|ovs-dpdk)` | `(tier-0\|\|tier-1) && !hypershift` |
| **Nightly Filter** | `ovs-dpdk && !hypershift` | `tier-2 && !hypershift && !release-critical` |
| **Semantic** | ❌ "Not ovs-dpdk" | ✅ "Component functional" |
| **ovs-dpdk Coverage** | ❌ 0 tests in fast lane | ✅ 1 critical test in fast lane |
| **Tier2 Coverage** | ⚠️ Runs in fast lane (mixed) | ✅ Runs in Tier2 lane (clean) |
| **Clarity** | ❌ Feature-specific | ✅ Test-level-based |

---

## Recommendation

**Adopt the tier-based approach:**

1. ✅ Re-label test_id:89987 as Tier1 (basic ovsDpdk functionality)
2. ✅ Keep test_id:89988/89994/89997 as Tier2 (integration/lifecycle)
3. ✅ Update Makefile to use tier-based filters
4. ✅ Rename lanes: `pao-functests-updating-profile` → `pao-functests-tier1`, `pao-functests-updating-nightly` → `pao-functests-tier2`

**Result:**
- Tier1 lane: ~161 min, includes critical ovsDpdk test
- Tier2 lane: ~85 min, comprehensive integration coverage
- Clear, semantic, maintainable
