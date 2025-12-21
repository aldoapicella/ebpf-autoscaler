# WSL2 measurement validity (detailed)

WSL2 introduces a full virtualization layer (Hyper-V) underneath your Linux kernel. That layer can add noise to scheduler timing, networking, and CPU accounting. The runqueue signal remains useful for relative/causal insights, but absolute numbers (latency in ms, overhead percentages) can drift compared to native Linux. Use WSL2 to develop and validate the method; run final quantitative claims on native Linux.

## How WSL2 differs from native Linux
- Two schedulers: Windows schedules the VM; the VM schedules your containers. Host load, power management, and vCPU pinning decisions can inject latency/jitter your app never requested.
- Virtual networking stack: kind already adds a container network; WSL2 adds NAT/vSwitch and host-side packet handling, increasing RTT baseline and variability.
- Timer/vCPU accounting: virtual timers and vCPU scheduling can skew CPU time and wakeup timing versus bare metal.

## Impact by metric
- Runqueue latency (`runqueue_latency_ms_*`): shapes and ordering are reliable (it rises under load, precedes CPU%), but p50/p95 values can be inflated or more bursty due to host scheduling.
- TCP RTT / retrans: baselines are higher and more variable because packets traverse the VM boundary and virtual switches.
- CPU overhead of eBPF: measured overhead may differ—sched_switch frequency and virtualization effects can change ringbuf and program cost versus native.

## Safe claims on WSL2
- End-to-end correctness: eBPF → Prometheus → Grafana → Adapter/HPA works.
- Relative behavior: signal leads CPU average/app p95; reacts to bursts earlier than conventional metrics; controller logic can consume it.
- Pipeline tuning: sampling rates, bucket choices, cardinality, scrape intervals, dashboards.

## Claims that need caveats
- Absolute latency numbers (e.g., runqueue p95 in ms) when compared to production expectations.
- Overhead expressed as a fixed percent ("<2% CPU") without native confirmation.

## Claims to avoid on WSL2
- Production-like performance deltas (e.g., "p95 app from 180ms to 120ms"), or fine microbenchmarks between techniques.

## Recommended workflow
- Develop on WSL2: iterate on code, collectors, dashboards, and controller logic.
- Validate on native Linux for numbers: rerun key experiments (HPA vs proactive vs RL, overhead, tail latency) on a dedicated Linux VM, bare metal, or cloud VM. No code changes required—only environment.
- Document threats to validity: explicitly note WSL2/kind virtualization for scheduling and networking and point to native runs for absolute figures.

## Practical checks to gauge WSL2 noise
- Idle jitter check: observe runqueue metrics at idle; large intermittent spikes suggest host scheduling interference.
- Repeatability: run 5 identical k6/stress bursts; compare runqueue p95 and app p95; compute coefficient of variation (CV). High CV implies virtualization noise.
- Baseline RTT: measure TCP RTT between pods; if baseline is already elevated/variable, expect corresponding noise in latency-sensitive metrics.

## How to report in thesis or docs
- State scope: "Development on WSL2/kind; quantitative results validated on native Linux."
- Mark caveats: call out that scheduling and networking measurements on WSL2 can be noisier or inflated.
- Separate findings: use WSL2 for demonstrating signal lead/causality; use native runs for absolute latency/overhead claims.
