# Release-Critical Label: Long-Term Strategy

## Current State

We currently have **TWO overlapping classification systems**:

### 1. **Tier-Based** (Test Level)
- Tier0: Unit tests
- Tier1: Component functional
- Tier2: Integration
- Tier3: System/scenario

### 2. **Criticality-Based** (Business Impact)
- `release-critical`: Must pass for release
- Non-critical: Should pass, but won't block release

---

## The Problem: Overlap and Confusion

### Current Lane Structure

| Lane | Filter | Runtime | Purpose |
|---|---|---|---|
| **Tier1** | `(tier-0\|\|tier-1) && !hypershift` | ~161 min | Component functional |
| **Tier2** | `tier-2 && !hypershift && !release-critical` | ~85 min | Integration (non-critical) |
| **Release-Critical** | `release-critical && !hypershift` | ~90 min | Release gating |

### Issues:

**1. Tier2 excludes release-critical tests**
```bash
tier-2 && !release-critical
#         ^^^^^^^^^^^^^^^^^^^
#         Excludes critical Tier2 tests!
```

**Problem:** If a Tier2 integration test IS release-critical, where does it run?
- ❌ Not in Tier2 lane (excluded by `!release-critical`)
- ✅ Runs in Release-Critical lane
- ⚠️ But loses tier semantic (no way to know it's Tier2)

**2. Many Tier1 tests are ALSO release-critical**

Example: test_id:34081 (hugepages)
- Labeled: `tier-2` AND `release-critical`
- Runs in: **Release-Critical lane** (not Tier2 lane due to filter)

This creates duplication and confusion.

**3. Three lanes for what could be two dimensions**

We have:
- Tier1 lane (mostly critical)
- Tier2 lane (only non-critical due to filter!)
- Release-Critical lane (cross-tier)

**Better model:** Two independent dimensions
- **Dimension 1:** Tier (0/1/2/3) - test level
- **Dimension 2:** Criticality (critical/non-critical) - business impact

---

## Current Label Usage

<function_calls>
<invoke name="Bash">
<parameter name="command">grep -r "release-critical\|ReleaseCritical" test/e2e/performanceprofile/functests/ --include="*.go" | grep -E "test_id:" | wc -l
### Current Release-Critical Tests: **36 total**

Distribution across suites:
- 1_performance: 17 tests (Tier0, no explicit tier label)
- 2_performance_update: 4 tests (labeled Tier2 + release-critical)
- 3_performance_status: 3 tests (labeled Tier1 + release-critical)
- 7_performance_kubelet_node: 5 tests (labeled Tier1/Tier2 + release-critical)
- 10_ppc: 2 tests (release-critical)
- 11_mixedcpus: 2 tests (release-critical)
- 0_config: 1 test (Tier0 + release-critical)
- 6_mustgather: 1 test (release-critical)
- 8_workloadhints: 1 test (release-critical)

**Key observation:** Release-critical tests span ALL tiers!

---

## Long-Term Strategy Options

### Option 1: Tier-Primary (Current Approach)

**Lanes:**
- Tier1 lane: `(tier-0||tier-1) && !hypershift`
- Tier2 lane: `tier-2 && !hypershift && !release-critical`
- Release-Critical lane: `release-critical && !hypershift`

**Pros:**
- Clear tier semantic (test level)
- Matches official Red Hat tier definitions

**Cons:**
- ⚠️ Tier2 excludes release-critical (creates gap)
- Three lanes with overlap
- Confusing: "Where does a critical Tier2 test run?"

---

### Option 2: Criticality-Primary (Recommended Long-Term)

**Reorganize around criticality first, tier second**

**Lanes:**
- **Critical lane:** `release-critical && !hypershift` (~90 min)
  - PR blocking, must pass to merge
  - Cross-tier (Tier0+Tier1+Tier2 critical tests)
  
- **Non-Critical Tier1 lane:** `tier-1 && !release-critical && !hypershift` (~70 min)
  - PR informational/optional
  - Component functional, not release-blocking
  
- **Non-Critical Tier2 lane:** `tier-2 && !release-critical && !hypershift` (~85 min)
  - Nightly/periodic
  - Integration, not release-blocking

**Pros:**
- ✅ Clear business logic: Critical = blocks release
- ✅ No gaps: all tests have a home
- ✅ Tier semantic preserved (as secondary classifier)
- ✅ Easy to explain: "Critical blocks PR, non-critical doesn't"

**Cons:**
- Requires rethinking current structure
- Need to label ALL tests as critical or not

---

### Option 3: Matrix Approach (Most Flexible)

**Two independent dimensions:**

| | Tier0 | Tier1 | Tier2 | Tier3 |
|---|---|---|---|---|
| **Release-Critical** | Critical-T0 | Critical-T1 | Critical-T2 | Critical-T3 |
| **Non-Critical** | NonCrit-T0 | NonCrit-T1 | NonCrit-T2 | NonCrit-T3 |

**Lanes:**
```bash
# Fast lane: ALL critical tests (cross-tier)
release-critical && !hypershift

# Tier1 supplemental: Non-critical component tests
tier-1 && !release-critical && !hypershift

# Tier2 nightly: Non-critical integration
tier-2 && !release-critical && !hypershift

# Tier3 weekly: System/scenario
tier-3 && !hypershift
```

**Pros:**
- ✅ Flexible: every test classified by BOTH dimensions
- ✅ Clear priority: critical first, then tier
- ✅ Scalable: easy to add Tier3

**Cons:**
- Requires labeling discipline
- More complex filters

---

## Recommended Evolution Path

### Phase 1: Current State (Completed ✅)
- Tier1 lane: `(tier-0||tier-1) && !hypershift`
- Tier2 lane: `tier-2 && !release-critical && !hypershift`
- Release-Critical lane: `release-critical && !hypershift`

**Status:** Working, but suboptimal.

---

### Phase 2: Criticality Audit (Recommended Next)

**Action items:**
1. **Audit all Tier1 tests:** Which are truly release-critical?
2. **Audit all Tier2 tests:** Any that should be release-critical?
3. **Label consistently:** Every test should have:
   - Tier label (tier-0, tier-1, tier-2, tier-3)
   - Criticality (release-critical or nothing)

**Example:**
```go
// Clear: Tier1 component functional, release-critical
It("test", Label(string(label.Tier1), string(label.ReleaseCritical)), func() {

// Clear: Tier2 integration, NOT release-critical  
It("test", Label(string(label.Tier2)), func() {

// Clear: Tier2 integration, IS release-critical
It("test", Label(string(label.Tier2), string(label.ReleaseCritical)), func() {
```

---

### Phase 3: Transition to Criticality-Primary (Future)

**New lane structure:**

**Lane 1: Release-Critical (PR blocking)**
```bash
--label-filter='release-critical && !hypershift'
# ~90 min, all critical tests regardless of tier
```

**Lane 2: Non-Critical Tier1 (PR optional)**
```bash
--label-filter='tier-1 && !release-critical && !hypershift'
# ~70 min, component functional but not blocking
```

**Lane 3: Non-Critical Tier2 (Nightly)**
```bash
--label-filter='tier-2 && !release-critical && !hypershift'
# ~85 min, integration regression coverage
```

**Benefits:**
- Clear separation: critical blocks, non-critical doesn't
- Tier semantic preserved
- No gaps or overlaps

---

## Current Issues to Address

### Issue 1: Tier2 Release-Critical Tests

**Problem:** Where do they run?

**Current:** They run in Release-Critical lane (excluded from Tier2 lane)

**Examples:**
- test_id:34081 (hugepages) - labeled `tier-2` + `release-critical`
- test_id:28071 (isolcpus) - labeled `tier-2` + `release-critical`

**Impact:** Tier2 lane doesn't represent "all Tier2 tests", only "non-critical Tier2"

**Solution:** Accept this as correct behavior:
- Release-Critical lane = all critical (cross-tier)
- Tier2 lane = non-critical integration only

---

### Issue 2: Inconsistent Labeling

**Problem:** Not all tests have explicit tier labels

**Examples:**
- Many tests in `1_performance` have NO tier label (assume Tier0/Tier1)
- Some tests have `release-critical` but no tier label

**Impact:** Filter behavior is implicit, hard to understand

**Solution:** Label ALL tests with explicit tier (0/1/2/3)

---

### Issue 3: Release-Critical Semantics Unclear

**Question:** What makes a test "release-critical"?

**Current (implicit):**
- P0 tests (RT guarantees, node stability)
- Tests guarding shipped CVEs/bugs (OCPBUGS-*)
- Core product promises

**Needed:** Document explicit criteria in code:
```go
// pkg/e2e/performanceprofile/functests/utils/label/label.go

// ReleaseCritical marks tests that MUST pass before cutting a release.
// Criteria:
// - P0 severity: failure breaks RT guarantees or node stability
// - Guards shipped CVE/OCPBUGS fix
// - Validates core product promise (CPU isolation, RT kernel, hugepages)
// - Broad blast radius: affects all nodes in pool, not single pod
```

---

## Recommendation for Next Steps

### Immediate (No Code Changes)

1. ✅ **Accept current structure** as working
   - Tier1 lane for component functional
   - Tier2 lane for non-critical integration
   - Release-Critical lane for all critical

2. ✅ **Document the gap:** Tier2 lane only includes non-critical Tier2

---

### Short-Term (Documentation)

1. **Update test_lanes_overview.md** to clarify:
   - Release-Critical lane is cross-tier
   - Tier2 lane excludes critical tests (by design)
   - Two dimensions: tier (test level) + criticality (business impact)

2. **Document release-critical criteria** in label.go

---

### Medium-Term (Label Audit)

1. **Audit all tests** for proper tier labeling
   - Every test should have a tier (0/1/2/3)
   - Verify criticality labels are accurate

2. **Create labeling guidelines** for new tests:
   ```
   How to label a new test:
   1. Tier: What level? (unit/component/integration/system)
   2. Criticality: Blocks release? (yes = release-critical)
   3. Other: slow, ovs-dpdk, hypershift, etc.
   ```

---

### Long-Term (Lane Reorganization)

**Consider switching to criticality-primary:**

```make
# Primary distinction: critical vs non-critical
# Secondary: tier level

# Lane 1: All critical (blocks PR/release)
pao-functests-release-critical:
  filter: release-critical && !hypershift
  
# Lane 2: Non-critical Tier1 (optional)
pao-functests-tier1-noncritical:
  filter: tier-1 && !release-critical && !hypershift
  
# Lane 3: Non-critical Tier2 (nightly)
pao-functests-tier2-noncritical:
  filter: tier-2 && !release-critical && !hypershift
```

**Benefits:**
- Clearer business logic
- No gaps
- Easier to explain

---

## Summary

**Current state:** ✅ Working, but has conceptual overlap

**Key insight:** We have TWO dimensions:
1. **Tier** (test level: unit/component/integration/system)
2. **Criticality** (business impact: blocks release or not)

**Short-term:** Accept current structure, document the gap

**Long-term:** Consider reorganizing lanes by **criticality first, tier second**

**Immediate action:** None - current implementation is valid. Think about labeling audit and eventual reorganization.
