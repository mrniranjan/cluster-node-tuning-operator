# Extended Analysis: Reboot-Heavy Release-Critical Tests

## Executive Summary

**Objective:** Identify which reboot-heavy tests in `2_performance_update` and `7_performance_kubelet_node` are truly release-critical (would block a release if they fail) vs. valuable but not blocking.

**Key Finding:** Only **4 reboot tests** (out of 91 total in the serial lane) are both:
1. Already marked `label.ReleaseCritical` in code
2. Trigger MCP rolls/reboots (expensive)
3. Actually execute on 4-CPU VMs (not skipped)

These 4 tests consume **~18 minutes** of the 220-minute serial lane.

---

## 1. Current ReleaseCritical Label Usage

The label exists (`utils/label/label.go:16`) and is already applied to **36 tests** across all suites:

| Suite | ReleaseCritical Count | Reboot Cost |
|---|---|---|
| **1_performance** | 17 | **0 min** (no-reboot by design) |
| **2_performance_update** | 4 | **~18 min** (4 reboot specs) |
| **3_performance_status** | 3 | **~1 min** (read-only) |
| **7_performance_kubelet_node** | 5 | **~1 min** (read-only cgroup checks) |
| 11_mixedcpus | 2 | (not in serial lane) |
| 10_ppc | 2 | ~0 min (offline tool) |
| 6_mustgather | 1 | (not time-critical) |
| 8_workloadhints | 1 | (not time-critical) |
| 0_config | 1 | **9 min** (mandatory prereq) |

**Total reboot cost of all release-critical tests:** ~28 minutes (13% of 220-minute lane).

---

## 2. Reboot-Heavy Tests in 2_performance_update — Release-Critical Assessment

### 2.1 Already Marked ReleaseCritical (4 tests, ~18 min)

These are **correctly tagged** and should **remain in time-boxed lane**:

| test_id | Description | File:Line | Reboot? | Time | Why Release-Critical |
|---|---|---|---|---|---|
| **34081** | Hugepages cmdline + allocation | updating_profile.go:308 | Yes (shared) | ~12 min (amortized) | Hugepages kernel args wrong → DPDK/RT workloads fail. **P0** kernel boot param. |
| **28071** | isolcpus + managed_irq | updating_profile.go:331 | Yes (shared) | ~12 min (amortized) | Incorrect CPU isolation → RT guarantees broken. **P0** TuneD bootloader. |
| **28071** | systemd.cpu_affinity | updating_profile.go:348 | Yes (shared) | ~12 min (amortized) | Systemd on isolated CPUs → RT latency spikes. **P0** TuneD bootloader. |
| **28935** | reservedSystemCPUs (kubelet) | updating_profile.go:380 | Yes (shared) | ~12 min (amortized) | Wrong kubelet CPU reservation → node instability. **P0** kubeletconfig. |
| **27738** | RT kernel toggle | updating_profile.go:395 | Yes | ~18 min | RT kernel not applied → real-time workloads fail. **P0** machineconfig. |

**Note:** Tests 34081/28071/28071/28935 share a **single MCP roll** via the `Ordered` context (Context "Verify that all performance profile parameters can be updated", updating_profile.go:225). They cost ~12 minutes **total**, not per test.

### 2.2 Expensive Non-Critical Reboot Tests (candidates to DROP from serial lane)

| test_id | Description | File:Line | Time | Why NOT Release-Critical |
|---|---|---|---|---|
| **ovsdpdk suite** | 4 specs in 2 Ordered blocks | ovsdpdk.go:138,267,371,414 | **~49 min** | **Telco-specific DPDK feature.** ovsDpdk is opt-in, non-default. Failure doesn't block general RT/perf releases. Label: `Tier2`, `Slow`. **Move to dedicated telco lane.** |
| **28440** | nodeSelector move to different MCP | updating_profile.go:547 | **~16 min** | Needs ≥2 spare worker nodes (often unavailable on CI). Tests MCP retargeting, not core RT tuning. **Tier2**. Move to nightly/infra lane. |
| **27484** | nodeSelector revert (remove labels) | updating_profile.go:558 | **~15 min** | Reversal of 28440. Same reasoning. **Tier2**. Move to nightly. |
| **22764** | No-op profile update | updating_profile.go:418 | ~18 min | Mostly revert cost from prior test. Validates idempotency (low-priority regression guard). **Tier2**. Can drop. |
| **78116** | Tuned deferred (in-place, Always) | tuned_deferred.go:140 | ~3 min | Duplicate of 78115 (same DeferMode, different change type). 6 tuned_deferred specs → **trim to 3** (one per DeferMode). |

**Immediate serial-lane savings:** Drop ovsdpdk (−49 min) + nodeSelector pair (−31 min) = **−80 min → 220 → 140 min**, keeping all P0 assertions.

### 2.3 Critical But SKIPPED on 4-CPU VMs (coverage gap)

These are **marked ReleaseCritical** but **do not run** on the 4-CPU CI workers:

| test_id | Description | File:Line | Skip Reason | Impact if Regressed |
|---|---|---|---|---|
| **75327** | Cgroup cpuset reassign on scale-up (OCPBUGS-34812) | cpu_management.go:697 | Needs >10 CPU | **P0-adjacent.** Guards a real shipped regression. All perf pods get wrong cpuset after node scale event → RT broken. Needs ≥8-CPU baremetal lane. |

---

## 3. Reboot-Heavy Tests in 7_performance_kubelet_node — Release-Critical Assessment

Suite `7_performance_kubelet_node` has **5 ReleaseCritical tests**, but **all are read-only** (no reboots):

| test_id | Description | File:Line | Time | Component Tested |
|---|---|---|---|---|
| **45493** | kubelet cpuManagerPolicy not overridable | kubelet.go:97 | ~1 min | Ensures annotation doesn't override PAO's `static` policy (**P1** guard). |
| **64097** | OVS dynamic pinning activation file | cgroups.go:112 | <1 min | Activation file `/etc/crio/ovs-activation` exists (**P1** MC:373-409). |
| **73046** | OVN pod cpuset = all CPUs | cgroups.go:122 | <1 min | OVN control-plane pods not isolated (**P1** cgroup check). |
| **64100** | OVS process affinity | cgroups.go:261 | <1 min | OVS slice affinity matches config (**P1** dynamic pinning). |
| **64101** | GU pod modifies OVS affinity | cgroups.go:275 | <1 min | GU pod creation adjusts OVS affinity (**P1/P2** dynamic behavior). |

**Cost:** ~5 min total (all read-only cgroup/config checks after the suite's BeforeAll setup). **No reboots.**

---

## 4. Ordered Contexts — Shared Reboot Amortization

Three `Ordered` contexts in `2_performance_update/updating_profile.go` **share a single MCP roll** across multiple tests:

### 4.1 Context: "Verify that all performance profile parameters can be updated" (line 225)

**Single BeforeAll** modifies the profile → MCP rolls once → 12 assertions read the result.

| Entry/It | test_id | Assertion | Shared Reboot Time |
|---|---|---|---|
| Entry | **34081** | Hugepages cmdline | ~12 min (1 roll) |
| Entry | **28070** | Hugepages NUMA-unspecified | (shared) |
| Entry | — | Hugepages 1G count | (shared) |
| Entry | — | Hugepages 2M count | (shared) |
| Entry | — | CPU affinity mask (28025) | (shared) |
| Entry | **28071** | isolcpus | (shared) |
| Entry | **28071** | systemd.cpu_affinity | (shared) |
| Entry | **28760** | topologyManager | (shared) |
| It | **28935** | reservedSystemCPUs | (shared) |
| It | **27738** | RT kernel disable | **+18 min (toggle adds 2nd roll)** |
| It | **28612** | AdditionalKernelArgs | (shared with revert) |
| It | **22764** | No-op update | (shared with revert) |

**Pattern:** Efficient. One profile update → 12 validations. **Keep this pattern.**

RT kernel toggle (27738) triggers a **second reboot** (kernel switch), adding ~18 min.

### 4.2 Context: "Offlined CPU API" (line 652)

**Skipped** on 4-CPU VMs (needs >8 CPU). All 6 specs (50964–50970) skip. **0 min cost on CI.**

### 4.3 Tuned Deferred (tuned_deferred.go:36)

6 specs test 3 DeferModes × 2 change types. Each spec **reboots** (tuned annotation triggers MCP roll).

| test_id | DeferMode | Change Type | Time | Redundant? |
|---|---|---|---|---|
| **78115** | Always | first-time | ~3 min | **Keep** (canonical Always test) |
| **78116** | Always | in-place | ~3 min | **Drop** (duplicate DeferMode, minor variation) |
| **78117** | Update | first-time | ~3 min | **Keep** (canonical Update test) |
| **78118** | Update | in-place | ~3 min | **Drop** (duplicate) |
| **78119** | (no annotation) | first-time | ~3 min | **Keep** (default behavior) |
| **78120** | Never | in-place | ~3 min | **Drop or Keep** (Never mode is edge case) |

**Recommendation:** Trim 78116/78118 (duplicates) → **3 specs × 3 min = 9 min** (vs. current 6 × 3 = 18 min). Save **~9 min**.

---

## 5. What Makes a Test Release-Critical?

**Criteria** (from analysis + OCPBUGS references):

1. **Failure ships a P0 regression:** RT guarantees broken, node-level service broken, DPDK/hugepage workloads fail, or admission allows invalid config.
2. **Broad blast radius:** Affects all nodes in MCP/pool, not a single pod or niche feature.
3. **No workaround:** Admin cannot fix via manual config.
4. **Guards a shipped CVE/bug:** E.g., OCPBUGS-34812 (75327), OCPBUGS-26401 (74767), OCPBUGS-45112 (88711).

**Non-blocking failures** (Tier2, not release-critical):

- Tool correctness (ppc, mustgather detail)
- Niche features (ovsDpdk, offline-cpus, 2-NUMA memorymanager)
- Idempotency checks (22764 no-op update)
- Multi-worker infra tests (28440/27484 nodeSelector move)
- HW-specific (LLC, netqueues, latency thresholds)

---

## 6. Recommended Tagging — Release-Critical Reboot Tests

### 6.1 KEEP ReleaseCritical (already tagged, reboot-heavy)

| test_id | File:Line | Rationale |
|---|---|---|
| **34081** | updating_profile.go:308 | Hugepages kernel args → DPDK/RT workloads (P0 TuneD bootloader) |
| **28071** (isolcpus) | updating_profile.go:331 | CPU isolation domain (P0 TuneD bootloader, RT guarantee) |
| **28071** (systemd.cpu_affinity) | updating_profile.go:348 | Systemd off isolated CPUs (P0 TuneD bootloader, latency) |
| **28935** | updating_profile.go:380 | kubelet reservedSystemCPUs (P0 kubeletconfig, node stability) |
| **27738** | updating_profile.go:395 | RT kernel toggle (P0 machineconfig, real-time workloads) |

### 6.2 ADD ReleaseCritical (currently Tier2, but should block release)

**None in reboot-heavy set.** The existing 4-test set is correct. Most other update tests are **Tier2** (integration-level, nightly).

### 6.3 REMOVE from Serial Lane (non-critical, expensive)

| test_id | Suite/File | Cost | New Home |
|---|---|---|---|
| **ovsdpdk suite** | ovsdpdk.go (all) | −49 min | Dedicated telco/DPDK lane (opt-in feature) |
| **28440 + 27484** | updating_profile.go:547,558 | −31 min | Nightly infra lane (needs ≥2 spare workers) |
| **22764** | updating_profile.go:418 | −0.5 min | Can drop (idempotency, low value) |
| **78116** | tuned_deferred.go:140 | −3 min | Drop (duplicate of 78115) |
| **memorymanager.go** (all) | memorymanager.go | 0 min (already skip) | Already skips; document as 2-NUMA only |

**Total savings:** **−83 min → 220 → 137 min**, keeping all P0/P1 release guards.

---

## 7. Structural Coverage Gaps — Reboot Tests That Cannot Run on 4-CPU VMs

These **ReleaseCritical** tests skip on the VM lane but guard real regressions:

| test_id | Description | Skip Reason | OCPBUGS | Why It Matters |
|---|---|---|---|---|
| **75327** | Cgroup cpuset reassign | Needs >10 CPU | OCPBUGS-34812 | All perf pods wrong cpuset after scale → RT broken (P0-adjacent) |
| netqueues suite | NIC queue tuning | No multi-queue NIC | — | Per-node RX/TX steering wrong → latency regression (P1) |
| **4_latency** | Real oslat/cyclictest | Baremetal + long run | — | Core product promise (bounded latency) unverified in CI (P0 on baremetal) |

**Recommendation:** Add **≥8-CPU baremetal nightly lane** for these. CI PR lane cannot catch them.

---

## 8. Final Tagging Recommendations for Time-Boxed Lane

### Option A: Use Existing `release-critical` Label

**Pros:**
- Infrastructure already exists (`label.ReleaseCritical`, `--label-filter`)
- 4 reboot tests already correctly tagged
- Deterministic, CPU-count independent

**Cons:**
- Current label set includes **36 tests** (most are fast/read-only, but some expensive non-critical like ovsdpdk are NOT tagged)
- Would need to **untag** or **move to nightly** the non-critical expensive tests

**Action:**
```bash
# Time-boxed lane runs:
ginkgo --label-filter='release-critical' ...
# Nightly lane runs everything else:
ginkgo --label-filter='!release-critical' ...
```

### Option B: Create New `time-boxed` Label

**Pros:**
- Explicit "fits in 2-hour CI window" semantics
- Can include high-value Tier1 tests that aren't strictly release-blocking

**Cons:**
- One more label to maintain
- Overlaps with `release-critical`

**Verdict:** **Use Option A** (existing label). The 4 critical reboot tests are already tagged. Just **drop ovsdpdk + nodeSelector** from the serial lane Makefile target, regardless of labels.

---

## 9. Immediate Action Plan (No Code Changes)

1. **Modify `Makefile` target `pao-functests-updating-profile`:**
   ```diff
   - 0, 2, 3, 7, 9, 13
   + 0, 2*, 3, 7, 13
   ```
   Where `2*` = `2_performance_update` **excluding** `ovsdpdk.go` (add `--skip-file=ovsdpdk.go` or use `--label-filter=!(ovs-dpdk)`).

2. **Drop nodeSelector tests** (28440/27484) — they're labeled `tier-2, openshift` and conditional on spare workers. Add to nightly.

3. **Trim tuned_deferred** from 6 → 3 specs (keep one per DeferMode).

**Immediate savings:** **220 → 137 min** (37% reduction), keeping all P0 release guards.

---

## 10. Code-Level Changes (Future Work)

### 10.1 Delete Dead Tests
- **36364** (cpu_management.go:430) — unconditional `Skip()`, never runs
- **GetNumaRanges** (nodes_test.go:11) — unit test mislocated as e2e

### 10.2 Label Dark Tests
Add `label.SpecializedHardware` to tests that skip on 4-CPU VMs:
- All `13_llc` functional/runtime (baremetal uncore-cache)
- `75327` (>10 CPU)
- `netqueues` suite (multi-queue NIC)
- `4_latency` (baremetal + thresholds)

### 10.3 Deduplicate
- Legacy v1/v1alpha1 webhook tests (keep v2 only)
- LLC odd-CPU specs (87072–87074, AI-generated near-tautologies)

---

## 11. Summary Table — Reboot Test Criticality Matrix

| Test | Suite | Reboot Cost | 4-CPU VM? | Release-Critical? | Action |
|---|---|---|---|---|---|
| **34081** hugepages | 2_update | 12 min (shared) | ✅ | ✅ **P0** | **KEEP** |
| **28071** isolcpus | 2_update | 12 min (shared) | ✅ | ✅ **P0** | **KEEP** |
| **28071** systemd.cpu_affinity | 2_update | 12 min (shared) | ✅ | ✅ **P0** | **KEEP** |
| **28935** reservedSystemCPUs | 2_update | 12 min (shared) | ✅ | ✅ **P0** | **KEEP** |
| **27738** RT kernel toggle | 2_update | 18 min | ✅ | ✅ **P0** | **KEEP** |
| **ovsdpdk** (4 specs) | 2_update | **49 min** | ✅ | ❌ Telco opt-in (Tier2) | **DROP from serial** |
| **28440** nodeSelector move | 2_update | **16 min** | ⚠️ Needs spare workers | ❌ Infra test (Tier2) | **DROP from serial** |
| **27484** nodeSelector revert | 2_update | **15 min** | ⚠️ Needs spare workers | ❌ Infra test (Tier2) | **DROP from serial** |
| **22764** no-op update | 2_update | 0.5 min | ✅ | ❌ Idempotency (Tier2) | **Can drop** |
| **78116/78118** tuned (dupes) | 2_update | 6 min | ✅ | ❌ Duplicate tests | **Trim to 3 specs** |
| **45493** kubelet override | 7_kubelet | <1 min | ✅ | ✅ **P1** | **KEEP (no reboot)** |
| **64097/73046/64100/64101** OVS | 7_kubelet | ~4 min | ✅ | ✅ **P1** | **KEEP (no reboot)** |
| **75327** cgroup reassign | 1_perf | 0 (SKIP) | ❌ >10 CPU | ✅ **P0 bug guard** | **Needs ≥8-CPU lane** |

**Current serial lane:** 220 min, 91 specs.  
**After drops:** **137 min**, 74 specs, all P0/P1 guards retained.
