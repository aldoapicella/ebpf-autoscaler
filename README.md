# ebpf-autoscaler

Runqueue-aware autoscaling sandbox for Kubernetes on Kind. An eBPF collector attaches to raw scheduler tracepoints (`sched_wakeup`, `sched_switch`) to export runqueue latency histograms. Metrics are scraped by Prometheus, visualized in Grafana, and can feed Prometheus Adapter for scaling experiments.

## Repository layout
- `collector-ebpf/`: Go exporter, eBPF C programs, and bpf2go bindings. Raw tracepoints are used instead of perf-event links (WSL2 perf attach was denied).
- `infra/kind/`: Kind multi-node cluster config with mounts for BTF and kernel modules.
- `infra/helm/`: Helm values for kube-prometheus-stack and Prometheus Adapter.
- `infra/k8s/collector-ebpf/`: Namespace, DaemonSet, Service, ServiceMonitor, Grafana dashboard ConfigMap.
- `docs/`: Troubleshooting and environment notes (`docs/bug-runqueue-perf-permission.md`, `docs/wsl2-measurement-validity.md`).
- `STATUS.md`: Current system status and validations.

## Prerequisites
- Linux host with Docker and Kind.
- Kernel exposing BTF at `/sys/kernel/btf/vmlinux` (needed to generate `collector-ebpf/bpf/vmlinux.h`).
- `make`, `bpftool`, `clang/llvm`, `libbpf-dev`, `gcc`, `make`. The `make dev-up` path installs missing bpftool/clang via apt on Debian/Ubuntu.

## Quick start (Kind + observability + collector)
```bash
# clean and recreate cluster + monitoring
kind delete cluster --name ebpf-scale || true
make dev-up            # installs Kind/Helm if missing, brings up cluster, metrics-server, kube-prom-stack, adapter, ensures bpftool/clang
make collector-build   # generates bpf/vmlinux.h from host BTF, then builds image collector-ebpf:dev
make collector-load    # loads image into Kind nodes
make collector-up      # applies namespace, DaemonSet, Service, ServiceMonitor, dashboard

# check
kubectl get nodes
kubectl -n monitoring get pods
kubectl -n ebpf get pods,svc
```

## Observability
- Prometheus UI: `kubectl -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090:9090`
- Grafana UI: `kubectl -n monitoring port-forward svc/kps-grafana 3000:80` (admin/admin); dashboard auto-loaded from `infra/k8s/collector-ebpf/40-grafana-dashboard.yaml`.
- Collector metrics directly: `kubectl -n ebpf port-forward svc/collector-ebpf 2112:2112 && curl -s localhost:2112/metrics | head`

## Key metrics & PromQL
- Runqueue histogram: `runqueue_latency_ms_bucket/count/sum` emitted per node (`exported_pod` label) from raw tracepoints.
  - Event rate per node: `sum by (exported_pod)(rate(runqueue_latency_ms_count[1m]))`
  - p95: `histogram_quantile(0.95, sum by (le, exported_pod)(rate(runqueue_latency_ms_bucket[5m])))`

## Regenerating eBPF artifacts
- `collector-ebpf/bpf/vmlinux.h` is generated (gitignored) by `make collector-build` using `bpftool btf dump file /sys/kernel/btf/vmlinux format c`.
- If the host lacks BTF (`CONFIG_DEBUG_INFO_BTF`), generation will fail; use a kernel with BTF enabled or supply a matching `vmlinux.h` manually.

## WSL2/kind notes
- Perf-event attach was denied on WSL2; collector uses raw tracepoints to avoid perf_event constraints.
- WSL2 can distort absolute latency/overhead; use it for development and validate final numbers on native Linux. See `docs/wsl2-measurement-validity.md`.

## Cleanup
```bash
make dev-down   # deletes Kind cluster
```

## Next steps
- Add real `queue_depth` and `tcp_rtt_ms` metrics (placeholders today).
- Wire Prometheus Adapter rules for runqueue-based scaling signals.
- Harden DaemonSet security (read-only rootfs, tightened caps) once WSL2 constraints are handled.
