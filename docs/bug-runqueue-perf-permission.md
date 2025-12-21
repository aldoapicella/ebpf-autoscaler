# Runqueue perf-link denial on kind/WSL2 (resolved)

Status: fixed. We stopped using perf-event links (which returned `EACCES` for `sched_switch` on WSL2) and switched to raw tracepoints. Collector now runs and emits runqueue metrics. Measurement caveats for WSL2 live in `docs/wsl2-measurement-validity.md`.

## Impact
- Before fix: `collector-ebpf` CrashLoopBackOff; no runqueue metrics exported.
- After fix: DaemonSet pods `Running`; runqueue metrics visible in Prometheus/Grafana; autoscaler inputs unblocked.

## Environment
- Cluster: kind `ebpf-scale` on WSL2, kernel `5.15.146.1-microsoft-standard-WSL2`.
- Component: `collector-ebpf` DaemonSet (runqueue programs on `sched_wakeup`/`sched_switch`).
- Image/security: `collector-ebpf:dev` (distroless, user 0), privileged with `BPF, PERFMON, SYS_ADMIN, SYS_RESOURCE`; `seccomp: Unconfined`; hostPID; mounts `/sys/kernel/{btf,tracing}`, `/sys/fs/{bpf,cgroup}`, `/lib/modules`; init container sets `perf_event_paranoid=-1`.

## Timeline (Dec 20, 2025)
- Failure: `BPF_LINK_CREATE` for perf-event attach (`sched_switch`) returned `EACCES`; perf_event ioctl fallback also denied, causing CrashLoopBackOff.
- Attempted mitigation: set `unprivileged_bpf_disabled=0` on nodes; no change.
- Fix: replaced perf-event attach with raw tracepoints (`raw_tracepoint/sched_wakeup` and `raw_tracepoint/sched_switch` in `bpf/runqueue_rawtp.c`); Go uses `link.AttachRawTracepoint`; main logs and continues on attach failure to avoid CrashLoop.
- Outcome: rebuilt image, rolled DaemonSet; pods `Running`; stress test confirmed runqueue metrics flowing.

## Current verification
- Prometheus series present: `runqueue_latency_ms_count{job="collector-ebpf", exported_pod="ebpf-scale-worker"|"ebpf-scale-worker2"}` with non-zero counters.
- Grafana: panel "runqueue events per second" (`sum(rate(runqueue_latency_ms_count[1m]))`) shows traffic.
- Service scrape also confirms histograms at `http://collector-ebpf:2112/metrics`.

## Evidence from the failure
- Sysctls: `/proc/sys/kernel/perf_event_paranoid = -1`; `/proc/sys/kernel/unprivileged_bpf_disabled = 2`.
- Capabilities: all caps effective; seccomp unconfined.
- Tracepoints readable: `/sys/kernel/tracing/events/sched/sched_wakeup/id = 390`.
- Kernel config: `CONFIG_BPF_EVENTS=y`, `CONFIG_PERF_EVENTS=y`.
- Host perf works: `perf stat -e sched:sched_switch -a -- sleep 1` (~15k context switches).
- Strace in pod: `bpf(BPF_LINK_CREATE, ..., attach_type=BPF_PERF_EVENT) = -1 EACCES` even though `perf_event_open` fds were valid.
- Memlock inside pod: `ulimit -l = 65536`.

## Follow-ups
- Keep raw tracepoint attach path as default on WSL2/kind.
- For absolute performance/overhead claims, run experiments on native Linux (see `docs/wsl2-measurement-validity.md`).