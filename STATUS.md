# Project Status (Kind + Observability + eBPF collector)

## Current stack
- Kind cluster `ebpf-scale` (multi-node, BTF/lib/modules mounts) from `infra/kind/kind.yaml`.
- metrics-server installed (patched with `--kubelet-insecure-tls` for Kind).
- kube-prometheus-stack release `kps` in `monitoring` (values `infra/helm/kps-values.yaml`).
- Prometheus Adapter release `prom-adapter` in `monitoring` (values `infra/helm/adapter-values.yaml`).
- eBPF collector DaemonSet in `ebpf` namespace: Go 1.22 exporter (`collector-ebpf/cmd/collector/main.go`) exposing histograms `queue_depth` (dummy), `tcp_rtt_ms` (dummy), and **real** `runqueue_latency_ms` via **raw tracepoints** (`sched_wakeup` + `sched_switch`) with ringbuf; labels `namespace`, `pod`, `service`; image built via `collector-ebpf/Dockerfile` and loaded into Kind as `collector-ebpf:dev`.
- `Service` (`collector-ebpf`) and `ServiceMonitor` (`monitoring/collector-ebpf`) wired to Prometheus; Grafana dashboard ConfigMap at `infra/k8s/collector-ebpf/40-grafana-dashboard.yaml` auto-loaded by the kps dashboard sidecar.

## Repro steps
```bash
# fresh cluster + observability + collector
kind delete cluster --name ebpf-scale || true
make dev-up           # kind up + metrics-server + kube-prometheus-stack + adapter (installs bpftool/clang via bpf-deps)
make collector-build  # generates bpf/vmlinux.h from /sys/kernel/btf/vmlinux, then builds image
make collector-load   # load into kind
make collector-up     # apply namespace/DS/Service/ServiceMonitor/dashboard

# sanity checks
kubectl get nodes
kubectl -n monitoring get pods
kubectl -n ebpf get pods,svc

#+ Grafana (admin/admin)
kubectl -n monitoring port-forward svc/kps-grafana 3000:80

# Exporter metrics (optional)
kubectl -n ebpf port-forward svc/collector-ebpf 2112:2112
curl -s localhost:2112/metrics | head
```

## Validations (latest)
- Prometheus series present: `runqueue_latency_ms_count{job="collector-ebpf", exported_pod="ebpf-scale-worker"|"ebpf-scale-worker2"}` with non-zero counters; ServiceMonitor/Service in place, targets up.
- Pods `collector-ebpf-*` running on both workers with raw tracepoint attach confirmed in logs (`runqueue eBPF attached (sched_wakeup + sched_switch)`).
- Grafana dashboard (ConfigMap `infra/k8s/collector-ebpf/40-grafana-dashboard.yaml`) shows "runqueue events per second" panel moving under load.
- Build path regenerates `bpf/vmlinux.h` (ignored in git) from host BTF; `make dev-up` ensures bpftool/clang present.

## Next steps
- View dashboard: port-forward Grafana and open `eBPF Collector` dashboard to confirm runqueue p95 moves under load. (CONFIRMED for plumbing; verify movement after applying load.)
- Optional: add recording rules/PromQL for p95s and surface via Prometheus Adapter (HPAs).
- Optional: harden DS (read-only FS, tightened seccomp/capabilities) and implement real `queue_depth`/`tcp_rtt_ms` next.
