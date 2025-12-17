CLUSTER_NAME=ebpf-scale

.PHONY: dev-up dev-down kind-up kind-down obs-up metrics-server-up

dev-up: kind-up metrics-server-up obs-up
	@echo "✅ Cluster + monitoring listos"
	@kubectl get nodes
	@kubectl -n monitoring get pods

dev-down: kind-down

kind-up:
	kind create cluster --name $(CLUSTER_NAME) --config infra/kind/kind.yaml

kind-down:
	kind delete cluster --name $(CLUSTER_NAME)

metrics-server-up:
	# install oficial (latest) :contentReference[oaicite:3]{index=3}
	kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
	# en kind suele requerir insecure-tls para kubelet
	kubectl -n kube-system patch deploy metrics-server --type='json' -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]' || true

obs-up:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo update
	kubectl create ns monitoring --dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install kps prometheus-community/kube-prometheus-stack -n monitoring -f infra/helm/kps-values.yaml
	helm upgrade --install prom-adapter prometheus-community/prometheus-adapter -n monitoring -f infra/helm/adapter-values.yaml
