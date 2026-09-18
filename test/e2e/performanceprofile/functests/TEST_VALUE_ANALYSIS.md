# PerformanceProfile functests — test value & CI-lane analysis

> Goal: on GitHub CI the functests run on **4-CPU VM** workers with a **hard time
> limit**. Many specs skip on such small nodes. This document assesses every test
> suite for (1) criticality if it fails, (2) value added, (3) no-value / redundant
> tests, and (4) reboot/update wall-clock cost — so the time-boxed lane runs only
> the high-value / critical specs.

## 1. How the lanes are wired (Makefile)

Every lane prepends `0_config` (applies the profile the rest depend on).

| Make target | Dirs | Filter / notes |
|---|---|---|
| `pao-functests-only` | 0, **1**, **6**, **10** | main acceptance lane |
| `pao-functests-updating-profile` | 0, **2**, **3**, **7**, **9**, **13** | `--label-filter=!(hypershift)`, 5h — **the measured serial lane (~220 min)** |
| `pao-functests-update-only-hypershift` | 0, 2, 7, **8**, **12** | `--label-filter=!(openshift||slow)`, HCP only |
| `pao-functests-performance-workloadhints` | 0, **8** | |
| `pao-functests-latency-testing` | 0, **5** | needs ≥10 CPU |
| `pao-functests-mixedcpus` | 0, **11** | |
| `pao-functests-hypershift` | 0, 1, 3, 6 | `--label-filter=!openshift` |
| (arm target) | **14** | aarch64 only |

**Key finding:** `utils/label/label.go:16` already defines a `release-critical`
label — **but no test uses it.** Tiers exist too (Tier0:35, Tier1:7, Tier2:17,
Tier3:5). The selection mechanism for a critical lane already exists; it is unused.

## 2. Measured timing (serial `updating-profile` lane, build-log.txt)

Total **~219.6 min / 91 executed specs**. 88% is one suite.

| Suite/file | min | pass/skip |
|---|---|---|
| 2_performance_update/updating_profile.go | 112.9 | 17/10 |
| 2_performance_update/ovsdpdk.go | 49.0 | 4/0 |
| 2_performance_update/tuned_deferred.go | 13.9 | 6/0 |
| 7_performance_kubelet_node/kubelet.go | 12.6 | 5/0 |
| 13_llc/llc.go | 9.1 | 2/14 |
| 0_config | 9.0 | 1/0 |
| 9_reboot/devices.go | 6.6 | 1/2 (both device tests skip; time is setup) |
| 7_performance_kubelet_node/cgroups.go | 5.3 | 5/8 |
| 3_performance_status/status.go | 1.0 | 5/0 |
| 2_performance_update/memorymanager.go | 0.1 | 0/7 (all skip) |

The wall-clock is almost entirely MCP-roll / reboot cost, **not** assertion cost.

## 3. Critical set — failure = urgent / release-blocking

Node-level guarantees; a red here ships a real regression. All are cheap
read-only checks (no reboot) unless noted.

| Guarantee | test_id | file:line |
|---|---|---|
| CPU isolation correctness (broadest) | 37862 | 1_performance/cpu_management.go:136 |
| isolcpus + managed_irq | 28071 / 32702 | 2_performance_update/updating_profile.go:331 ; 1_performance/performance.go:188 |
| workqueue/writeback mask | 27081 | 1_performance/performance.go:200 |
| reserved CPU accounting | 28935 / 28528 | updating_profile.go:380 ; cpu_management.go:126 |
| CPU-manager static policy not overridable | 45493 | 7_performance_kubelet_node/kubelet.go:97 |
| RT kernel on/off | 26861 / 27738 | 1_performance/rt-kernel.go:47 ; updating_profile.go:395 |
| hugepages cmdline + allocated | 34081 | updating_profile.go:308 |
| workload partitioning (no procs on isolated) | 73107 / 87722 | performance.go:324 ; cpu_management.go:171 |
| **tuned not Degraded** (the production failure class) | – / 29673 / 40402 / ovsDpdk | performance.go:151 ; 3_performance_status/status.go:101,126,219 |
| one-shot before kubelet (OCPBUGS-26401) | 74767 | performance.go:346 |
| irqbalance restart-limit (OCPBUGS-45112) | 88711 | 1_performance/irqbalance.go:390 |
| cgroup cpuset reassignment (OCPBUGS-34812) | 75327 | cpu_management.go:697 (skips on 4-CPU) |
| topology manager policy | 26932 | 1_performance/topology_manager.go:38 |
| stalld running | 35363 | 1_performance/performance.go:237 |
| sysctl kernel/network params | 28466 / 28467 | performance.go:397 ; performance.go:558 |
| webhook v2 admission | – | performance.go:1078/1089/1100 |
| OVS activation + OVN cpuset (dynamic pinning) | 64097 / 73046 / 64100 / 64101 | 7_performance_kubelet_node/cgroups.go:112/122/261/275 |
| mixed-cpus reserved+shared / crio shared_cpuset | – | 11_mixedcpus/mixedcpus.go:95/109 |
| conflicting workload-hints rejected | 54184 | 8_performance_workloadhints/workloadhints.go:676 |
| PPC generator happy-path + per-pod-power | 40940 / 54187 | 10_performance_ppc/ppc.go:76/178 |
| must-gather captures PAO resources | – | 6_mustgather_testing/mustgather.go:60 |

## 4. Value per suite

| Suite | Value | Runs on 4-CPU VM? |
|---|---|---|
| 0_config | Prerequisite (applies the profile) | Yes (1 reboot, 9 min) |
| 1_performance | Core acceptance; highest critical density; **no reboots by design** | Yes (most) |
| 2_performance_update | Profile-mutation correctness; expensive | Partly |
| 3_performance_status | Status/Degraded propagation; cheapest high-signal suite | Yes |
| 7_performance_kubelet_node | Kubelet overrides + dynamic OVS pinning | Partly (8 skip) |
| 8_performance_workloadhints | Power/RT hint → kernel-arg tuning; each spec reboots | Partly |
| 11_mixedcpus | Shared-CPU feature; best coverage-per-reboot | Yes (adapts to ≤4 cores) |
| 13_llc | LLC/uncore-cache pinning | No (14/16 skip; baremetal CCX) |
| 4_latency | Real oslat/cyclictest/hwlatdetect | No (baremetal + thresholds) |
| 5_latency_testing | Meta-test of latency harness parsing | No (needs ≥10 CPU) |
| 10_performance_ppc | performance-profile-creator CLI; offline & cheap | Yes (podman+MUSTGATHER_DIR) |
| 6_mustgather | Support-bundle collection | Yes (pays full must-gather) |
| 9_reboot | Device-plugin recovery after reboot | No (env-gated, SRIOV HW) |
| 12_hypershift | HCP multi-nodepool profiles | No (needs HCP mgmt cluster) |
| 14_arm | ARM kernel page size + hugepages | No (skips on x86) |

## 5. No-value / drop / merge / relocate

### Dead or buggy
- `1_performance/cpu_management.go:430` (36364) — **unconditional `Skip()`**, never runs → delete.
- `7_performance_kubelet_node/nodes_test.go:11` (GetNumaRanges) — **unit test as e2e** → move to unit package.
- `6_mustgather_testing/mustgather.go:85` — dead `filepath.Walk` (result overwritten at :116); `:161` builds slice with leading empty strings (latent glob bug) → fix/clean.

### Low value / redundant (merge/drop)
- Legacy-API webhook+conversion in `1_performance/performance.go`: v1alpha1 trio (:960/971/982) and v1 trio (:1019/1030/1041) duplicate the live **v2** set (:1078/1089/1100). Keep v2 only. Same for conversion 35887 and the gdilb true/false pair.
- `13_llc` odd-CPU specs 87072/87073/87074 (:890/922/996) — AI-generated, near-tautological assertions, add 2 extra MCP cycles.
- `5_latency_testing` negative entries (42853/42852/42856) — validate test-tooling parsing → unit test.
- Mirror pairs: 50990↔50991, 54178↔54179, 54185↔54186 (workloadhints); mixedcpus best-effort↔burstable negatives; 72079↔72081 quota; RPS 59572→55012.

### Relocate to HW-specific lanes (pure skip cost on 4-CPU)
- All `13_llc` functional/runtime (15 specs) — baremetal uncore-cache.
- All `memorymanager.go` (7) — need 2 NUMA (already skip).
- 8 skipped `cgroups.go` specs — need >8 CPU or workload-partitioning.
- `4_latency`, `5_latency_testing`, `9_reboot`, `12_hypershift`, `14_arm` — wrong environment.

## 6. Reboot / update-heavy specs (time sinks)

| Spec / block | file:line | Cost | Time-boxed lane? |
|---|---|---|---|
| ovsdpdk (2 Ordered blocks) | ovsdpdk.go:138,267,371,414 | ~49 min | Drop → telco lane |
| nodeSelector move + revert (28440,27484) | updating_profile.go:547,558 | ~31 min | Drop (needs extra workers) |
| 22764 no-op update | updating_profile.go:418 | ~18 min (mostly shared revert) | Marginal (delete saves ~30s) |
| shared param-update ctx (12 reads / 1 roll) | updating_profile.go:225 | ~12 min | **Keep — efficient pattern** |
| tuned_deferred (3/6 reboot) | tuned_deferred.go:134,152,158 | ~14 min | Keep (trim dup 78116) |
| 13_llc BeforeAll/AfterAll | llc.go:104-205 | ~9 min | Trim to 1 config spec |
| 0_config profile deploy | config.go:48 | 9 min | Mandatory |
| single-HT / DRA / exec-affinity | updating_profile.go:1453,1772,1305 | ~6 min ea | Keep (unique) |

Reboot specs that already **skip** on 4-CPU (0 min, noise only): offline-CPU set,
RPS 56006, nosmt 86347, memorymanager, devices, 2-NUMA hugepage splits.

## 7. Recommendation

1. **Adopt the existing `release-critical` label.** Tag the Section-3 specs, run the
   time-boxed lane with `--label-filter='release-critical'`. Deterministic, CPU-count
   independent critical lane; everything else moves to nightly/HW lanes.
2. **Immediate serial-lane wins (no label work):** drop **ovsdpdk** (−49 min) and the
   **nodeSelector pair** (−31 min) → ~220 → ~140 min, keeping every P0 assertion.
3. **Relocate** 13_llc / latency / memorymanager / hypershift / arm / devices off the
   4-CPU lane (pure skip/setup cost there).
4. **Cleanup** dead `36364`, `GetNumaRanges` unit test, legacy v1/v1alpha1 dupes,
   must-gather dead code.

**Caveat:** several regression guards for real shipped bugs (e.g. 75327 /
OCPBUGS-34812) **skip on 4-CPU VMs** — a VM lane structurally cannot catch those. If
they matter, they need an ≥8-CPU lane regardless of time budget.

## 8. Code impact & blast radius — per executing spec (upstream CI)

This maps **every spec that actually executes** on the 4-CPU single-NUMA VM lanes to
the NTO source component it exercises, the production failure if it regresses, and the
blast radius (scope × severity). Skipped specs are listed at the end of each suite.

### 8.0 Which component owns what (the render pipeline)

Reconcile: `performanceprofile_controller.go:520 Reconcile` → `handler.go:35 Apply`
(CreateOrUpdate Tuned:106 / MC:136 / KC:142 / RuntimeClass:148) →
`manifestset.go:53 GetNewComponents` (machineconfig.New / kubeletconfig.New /
tuned.NewNodePerformance / runtimeclass.New). Node-side apply + bootcmdline reporting:
`pkg/tuned/controller.go:337`.

| Component (source) | Owns (what a failing test there implicates) |
|---|---|
| **TuneD template** `assets/performanceprofile/tuned/openshift-node-performance` (+intel/amd/rt), rendered by `components/tuned/tuned.go` | **All kernel cmdline args & RT tuning**: isolcpus/managed_irq, rcu_nocbs, nohz_full, `tuned.non_isolcpus` (workqueue mask), `systemd.cpu_affinity`, rcutree.kthread_prio, `[sysctl]` (sched_rt_runtime_us, nmi_watchdog, tcp_fastopen, dirty_ratio…), `[scheduler]` group prios, `[service] stalld`, `[cpu]` govern/cstate, `[net] channels=combined`, hugepage kargs, AdditionalKernelArgs |
| **machineconfig** `components/machineconfig/machineconfig.go` | RT-kernel switch (`KernelType`), per-NUMA hugepage systemd units, RPS/offline-CPU/clear-irqbalance systemd scripts, stalld-backend drop-in, OVS slice + dynamic-affinity activation file, one-shot ordering unit, **crio `99-runtimes.conf` snippet** (infra_ctr_cpuset, allowed_annotations, exec_cpu_affinity, GOMAXPROCS, shared_cpuset), mixed-cpus file |
| **kubeletconfig** `components/kubeletconfig/kubeletconfig.go` | reservedSystemCPUs, static CPUManager policy + reconcile period, TopologyManager policy, MemoryManager/ReservedMemory, evictionHard, full-pcpus-only, experimental-annotation pass-through (unsafe sysctls, GC, LLC option, DRA) |
| **runtimeclass** `components/runtimeclass/runtimeclass.go` | high-performance RuntimeClass used by crio pre-start hooks (cpuset.exclusive, load-balance/quota disable) |
| **profile / status / manifestset** | gating (mixed-cpus, RPS, ovsDpdk prereq), Degraded propagation to profile status, MCP retargeting |
| **pkg/tuned operand** `pkg/tuned/*` | deferred-update semantics (`always`/`update`/`never`), Applied/Deferred conditions, node-side apply |

**Severity key:** **P0** = node/RT/DPDK-broken or blocks whole lane; P1 = feature broken
on all pool nodes / silent regression; P2 = narrower or recoverable; P3 = tool/status/
support-bundle only. **Scope** = {single pod, single perf node, all nodes in MCP/pool,
whole cluster, API/admission-only, tool-only(offline), support-bundle-only}.

### 8.1 P0 set (node/RT/DPDK-broken; a red here breaks real-time guarantees)

None of these skip on the 4-CPU VM — the VM lane *does* exercise the RT-critical output.

| Guarantee | test_id | file:line | Component |
|---|---|---|---|
| isolated sysfs + pid1 `systemd.cpu_affinity` off isolated | 37862 / 31748 | cpu_management.go:136 ; performance.go:179 | TuneD [bootloader] :165 |
| isolcpus managed_irq / static-isolation domain | 32702 | performance.go:188 ; TMPL:172-176 | TuneD [bootloader] |
| workqueue mask `tuned.non_isolcpus` | 27081 | performance.go:200 | TuneD [bootloader] :165 |
| rcu_nocbs = isolated | 34358 | cpu_management.go:198 | TuneD [bootloader] :165 |
| rcutree.kthread_prio=11 | 54083 | performance.go:1114 | TuneD [bootloader] :178 |
| RT sysctls (sched_rt_runtime_us=-1 …) | 28466 | performance.go:397 | TuneD [sysctl] :79-146 |
| stalld running | 35363 | performance.go:237 | TuneD [service] :51 + MC stalld-backend |
| IRQ load-balancing disabled off isolated + preserved on tuned restart | 36150 / 86348 | irqbalance.go:86,173 | TuneD [irqbalance]/[scheduler] + crio MC:217 |
| load-balance-disable removes CPUs from sched domains | 32646 | cpu_management.go:902 | crio `cpu-load-balancing.crio.io` RT99:17 + RC |
| node points to correct tuned profile | 37127 | performance.go:112 | tuned recommend + asset include |
| RT kernel enabled on perf nodes | 26861 | rt-kernel.go:47 | MC:160-166 KernelType=RT |
| workload partitioning (no sys procs on isolated) | 73107 | performance.go:324 | crio 99-workload-pinning + OVS slice |
| reservedSystemCPUs correct (kubelet) | 28935 | updating_profile.go:380 | KC:161-182 |
| ovsDpdk isolation/partition + IRQ ban across pod churn | – | ovsdpdk.go:138,267,371,414 | tuned+KC+MC OVS (49 min; telco lane) |
| ovsDpdk prereq → Degraded when WP disabled | – | status.go:218 | profile.ValidateOvsDpdkCPUsPrerequisites |

### 8.2 1_performance (highest critical density; no reboots by design)

Beyond the P0 rows above, executing P1/P2 specs: 28528 reserved=cap−reserved
(KC:161/182, P1); 87722 infra-pod affinity (KC:123/182, P1); 27492 GU exclusive isolated
CPU (full-pcpus-only KC:226, P1); 73501 cpuset stable across kubelet restart
(OCPBUGS-43280, KC:123/182, P1); 49147 infra ctrs on reserved (`infra_ctr_cpuset` RT99:3,
P1); 49149/46959 SMT-aligned GU admission (full-pcpus-only KC:226, P1); 73382
kubepods cpuset.cpus.exclusive (OCPBUGS-34812, RC:12 pre-start hook, P1); 72079/72081
cpu-quota disable (RT99:17+RC, P1); 27752/34080/27477 hugepages NUMA/sizes/workloads
(TU:151+MC:260-285, P1); 42400/42696/379 stalld FIFO/prio/backend (TMPL:51+MC:444, P1/P2);
28611 additional kargs (module_blacklist=irdma, TU:169, P1); 74767 one-shot before kubelet
(OCPBUGS-26401, MC:362, P1); 28467 net-latency sysctls (TMPL, P1); 26932 topology policy
(KC:186, P1); 32364 second profile leaves primary MCP undisturbed (P1); 28526 non-perf node
has no RT kernel (MC:173, P1); **API layer**: v1/v1alpha1/v2 conversions + validating-webhook
reject (overlap/no-isolated/dup-selector; `v2/performanceprofile_validation.go:128/212/223`,
API/admission-only, P1); GOMAXPROCS injection ×7 (`min_injected_gomaxprocs` MC:692, P1/P2).
72080/37860 (single pod, P2); 54190/54083-RPS-off (P2).

*Skips on 4-CPU:* 36364 (unconditional Skip — **delete**), 46544 (SMT==1), 46538/46539
(SNO), 75327 (>10 CPU / OCPBUGS-34812), 83851/83856 (schedulable control plane), whole
**netqueues.go** suite (no multi-queue NIC on cloud VM: 40308/40543/40545/72051/40668),
59572/55012 (RPS off by default).

### 8.3 2_performance_update (profile-mutation; the expensive lane)

Hugepages cmdline+allocated 34081/28070 (TU:132-165 + MC:262-284, P1); **28025/28071
cpu-affinity mask + isolcpus + systemd.cpu_affinity (TuneD [bootloader], P0 RT)**;
28935 reservedSystemCPUs (KC, P0); 28760 topologyManager (KC:125/188, P1); 27738/22764
RT-kernel toggle (MC:160-167, P1/P2); 28612 kargs add/remove (TU:168, P1); 54191 RPS not
default (MC:71/200, P2); exec-cpu-affinity (runtimeclass+crio, P2); DRA managers off
(KC:71/114, P1); 28440/27484 nodeSelector move+revert (manifestset/handler, P2,
*conditional on ≥2 spare workers*). **ovsdpdk.go** all P0/P1 (see 8.1). **tuned_deferred.go**
6 specs → pkg/tuned operand (DeferMode annotations.go:13, status.go:112), all P2.

*Skips:* 45023/45024 (2 NUMA), offline-CPU Ordered 50964-50970 (>8 CPU), 56006 (>8 CPU),
86346/86347 (SMT), **all memorymanager.go** (2 NUMA).

### 8.4 3_performance_status (cheapest high-signal suite; ~1 min)

29673 MCP Degraded → profile (status.go:137, **P1**); 40402 Tuned Degraded → profile
(status.go:183, **P1**); status.go:218 ovsDpdk prereq Degraded (**P0**, see 8.1); 30894
Tuned name link (P3); 33791 runtimeClass name in status (P2). *Skip:* hypershift status
(not HCP).

### 8.5 7_performance_kubelet_node

kubelet.go: 45493 don't-override cpuManagerPolicy=static (KC:123, **P1**); 45488 kubelet
overrides pass-through (KC:57-61, P2); 45490 allocatable formula (KC:144-158, P2); 45495
topology policy (KC:185, P1, ARM-skip only); 45489 revert (P2). nodes_test.go:11
GetNumaRanges = **unit test mislocated as e2e** (no operator source, P3 — relocate).
cgroups.go dynamic OVS pinning: 64097/73046/64099/64098 activation-file + ovs.slice
(MC:373-409/404-408, **P1**); 64100/64101 OVN↔OVS affinity, GU pod modifies (P1/P2).

*Skips:* 64102/64103/75257 (>8 CPU), 89062-89066 (workload-partitioning / schedulable CP).

### 8.6 8_workloadhints / 11_mixedcpus / 13_llc

**8_workloadhints:** 50990/50991 RT-hint default (TU:228 + TMPL stalld/nohz_full/sysctl,
**P1**); 50992 HighPower (TU:235 + cstate kargs, P1); 50993 RT+HighPower idle=poll (P1);
54177 perPodPowerManagement → `[cpu] enabled=false` + pstate passive (TU:237, P1/P2);
**54184 both-hints rejected** (render guard TU:232, API/admission, P2). *Skips:* HW quartet
54178/54179/54185/54186 (real BIOS power-mgmt).

**11_mixedcpus** (fully runs on 4-CPU): mixed-cpus file MC:844/449 (P1); shared→
reservedSystemCPUs KC:174 (P1); crio shared_cpuset MC:676/688 (P1); disable-mixedCpus
gate profile.go:95-103 (P1); load-balance/quota/env/cgroup specs (single pod, P2);
admission guardrails burstable/best-effort/>1-resource/no-annotation (external mixedcpus
plugin, API-only P2/P3); exec-cpu-affinity ×4 (crio RT99:15, P2).

**13_llc** (only Configuration Tests run): 77722/77723/77724 `prefer-align-cpus-by-
uncorecache` via kubelet experimental annotation (KC:57-61, P2/P3). *Skips:* all functional/
runtime (77725-81673 — need baremetal + real uncore-cache) and odd-CPU 87072-87074.

### 8.7 0_config / 10_ppc / 6_mustgather

**0_config** = the profile every lane depends on → exercises the **whole controller
pipeline** (MC+KC+tuned+runtimeclass render + MCP roll). Failure blocks the entire lane.
Scope: all-MCP-nodes, **P0**. **10_ppc** performance-profile-creator CLI (profilecreator.go /
cmd/root.go validation) — **tool-only, offline, P2**. **6_mustgather** support-bundle
collection (gather-sysinfo.go) — **support-bundle-only, P3**; contains dead code
(mustgather.go:85 overwritten walk, :161 glob bug) → clean up.

### 8.8 Structural coverage gap (what a 4-CPU VM lane *cannot* catch)

Per-pod IRQ-disable (36364, unconditionally skipped anyway), SNO/WP HT scheduling
(46538/46539), the >10-CPU cgroup-reassign regression (75327 / OCPBUGS-34812),
control-plane pinning (83851/83856), all NIC netqueue validation (whole netqueues suite),
2-NUMA memory-manager & hugepage-split, and real-hardware BIOS power/latency
(8_workloadhints HW quartet, 4_latency). Also note **88711** (irqbalance StartLimitBurst,
OCPBUGS-45112) has **no producing source in this repo** — it asserts a base-RHCOS/MCO
property and can pass/fail on factors outside NTO.

## 9. Coverage gaps requiring urgent attention (E2E)

Two kinds of gap. **True absence** = the render/behaviour has *no* e2e assertion at
all. **Dark coverage** = an e2e spec exists but is unconditionally skipped on the
cloud/4-CPU VMs that upstream CI actually runs, so it never guards anything in CI.

| # | Area | NTO component | Gap kind | Why it's dark / absent | Blast radius if it regresses |
|---|------|---------------|----------|------------------------|------------------------------|
| 9.1 | **HardwareTuning** per-CPU freq cap (`Spec.HardwareTuning.IsolatedCpuFreq`/`ReservedCpuFreq` → tuned `[sysfs] .../scaling_max_freq`) | tuned template | **True absence** | Only an admission-validation unit test existed; the *render* had zero coverage (unit or e2e). No e2e ever sets the field. | Per-node: silently wrong/absent freq cap → power & thermal SLA miss on every perf node. P1. |
| 9.2 | **Netqueues** / `[net] channels=combined` | tuned template `[net]` | **Dark** | Entire netqueues suite skips on VMs lacking a multi-queue configurable NIC (the CI worker NIC). Render logic runs on real HW only. | Per-node NIC queue count wrong → RX/TX steering & latency regression. P1. |
| 9.3 | **75327 / OCPBUGS-34812** cgroup cpuset reassignment on scale-up | kubelet/CRI cgroup mgmt | **Dark** | Skips at ≤10 CPUs; 4-CPU CI worker never reaches the branch. Guards a real shipped regression. | All perf pods on a node get wrong cpuset after node scale event → RT guarantee broken. P0-adjacent. |
| 9.4 | **4_latency** real oslat/cyclictest/hwlatdetect | RT-kernel + full tuning stack | **Dark** | Needs baremetal + long run; skipped on VMs. The only *end-to-end* proof the RT path actually delivers latency. | The core product promise (bounded latency) is unverified in CI. P0 on baremetal, structurally un-runnable in the VM lane. |

### Recommendations (cheapest → most infra)

1. **HardwareTuning render unit test — done in this change** (`tuned_test.go`,
   `Context("with hardware tuning …")`). Cheapest immediate win: closes the true-absence
   gap at unit level (asserts `[sysfs]` `scaling_max_freq` entries for isolated cpus at
   `IsolatedCpuFreq` and reserved cpus at `ReservedCpuFreq`, plus a negative case). An
   e2e that actually sets the field on a real node is still worth adding for the
   apply-and-read-back path, but is HW-dependent.
2. **9.2 / 9.3 / 9.4 are HW/CPU-bound, not fixable by test authoring.** They need a
   dedicated **≥8-CPU + configurable-NIC baremetal lane** (nightly), not the 4-CPU VM
   PR lane. Until such a lane exists, treat these as *known-uncovered in CI* and rely on
   pre-merge manual/QE runs — do not assume green CI implies they pass.
3. **Label the dark specs** with a distinct HW/nightly label so the VM lane's skip count
   is honest and the gap is visible in reporting rather than hidden as "passed".
