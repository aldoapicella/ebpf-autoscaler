# Project Status (Kind + Observability Base)

## Current stack
- Kind cluster `ebpf-scale` created with multi-node config and BTF/lib/modules mounts (`infra/kind/kind.yaml`).
- metrics-server installed (patched with `--kubelet-insecure-tls` for Kind).
- kube-prometheus-stack installed as release `kps` in namespace `monitoring` (values in `infra/helm/kps-values.yaml`).
- Prometheus Adapter installed as release `prom-adapter` in `monitoring` (values in `infra/helm/adapter-values.yaml`).

## Repro steps
```bash
# create fresh cluster + observability
kind delete cluster --name ebpf-scale || true
make dev-up

# validate cluster
kubectl get nodes
kubectl -n monitoring get pods

# Grafana port-forward
kubectl -n monitoring port-forward svc/kps-grafana 3000:80
# login: admin / admin (password set in values)
```

## Observations
- Node versions: v1.30.0 (Kind default image).
- Initial pod states right after deploy may show `ContainerCreating`; wait for images to pull.

## Next steps
- Add DaemonSet eBPF collector (Go) exposing histograms `queue_depth`, `runqueue_latency_ms`, `tcp_rtt_ms` with labels `namespace`, `pod`, `service`.
- Add `Service` + `ServiceMonitor` for the collector.
- Create Grafana dashboards and Prometheus/adapter recording rules for p95 queries.
