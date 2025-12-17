# Project Status (Kind + Observability + eBPF collector)

## Current stack
- Kind cluster `ebpf-scale` (multi-node, BTF/lib/modules mounts) from `infra/kind/kind.yaml`.
- metrics-server installed (patched with `--kubelet-insecure-tls` for Kind).
- kube-prometheus-stack release `kps` in `monitoring` (values `infra/helm/kps-values.yaml`).
- Prometheus Adapter release `prom-adapter` in `monitoring` (values `infra/helm/adapter-values.yaml`).
- Dummy eBPF collector DaemonSet in `ebpf` namespace: Go 1.22 exporter (`collector-ebpf/cmd/collector/main.go`) exposing histograms `queue_depth`, `runqueue_latency_ms`, `tcp_rtt_ms` with labels `namespace`, `pod`, `service`; image built via `collector-ebpf/Dockerfile` and loaded into Kind as `collector-ebpf:dev`.
- `Service` (`collector-ebpf`) and `ServiceMonitor` (`monitoring/collector-ebpf`) wired to Prometheus; Grafana dashboard ConfigMap at `infra/k8s/collector-ebpf/40-grafana-dashboard.yaml` auto-loaded by the kps dashboard sidecar.

## Repro steps
```bash
# fresh cluster + observability + collector
kind delete cluster --name ebpf-scale || true
make dev-up           # kind up + metrics-server + kube-prometheus-stack + adapter
make collector-build  # build image
make collector-load   # load into kind
make collector-up     # apply namespace/DS/Service/ServiceMonitor/dashboard

# sanity checks
kubectl get nodes
kubectl -n monitoring get pods
kubectl -n ebpf get pods,svc

# Grafana (admin/admin)
kubectl -n monitoring port-forward svc/kps-grafana 3000:80

# Exporter metrics (optional)
kubectl -n ebpf port-forward svc/collector-ebpf 2112:2112
curl -s localhost:2112/metrics | head
```

## Validations (latest)
- Prometheus active targets show `collector-ebpf` scrape pool with two endpoints `10.244.1.7:2112` (worker) and `10.244.2.7:2112` (worker2) and `health:"up"`, no scrape errors.
- ServiceMonitor `collector-ebpf` present in `monitoring`; Service `collector-ebpf` present in `ebpf`.
- Collector pods running on both workers; image tag in pod metadata `collector-ebpf:dev`.
- Node version: v1.30.0 (Kind default image); other control-plane component metrics may show `connection refused` noise typical for Kind but unrelated to collector path.

## Next steps
- View dashboard: port-forward Grafana and open `eBPF Collector` dashboard to confirm panels render data.
- Optional: add recording rules/PromQL for p95s and surface via Prometheus Adapter (HPAs).
- Optional: harden DS (read-only FS, tightened seccomp/capabilities) once real eBPF code is added.
