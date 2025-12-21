CLUSTER_NAME=ebpf-scale
KIND_VERSION=v0.23.0
HELM_VERSION=v3.19.4

# eBPF build inputs
VMLINUX_BTF ?= /sys/kernel/btf/vmlinux
VMLINUX_H := collector-ebpf/bpf/vmlinux.h

.PHONY: dev-up dev-down deps kind-up kind-down obs-up metrics-server-up kind-install helm-install \
	collector-build collector-load collector-up collector-down bpf-deps

dev-up: deps kind-up metrics-server-up obs-up
	@echo "✅ Cluster + monitoring listos"
	@kubectl get nodes
	@kubectl -n monitoring get pods

dev-down: kind-down

deps: kind-install helm-install bpf-deps

# Install bpftool/clang if missing (for vmlinux.h generation and bpf2go)
bpf-deps:
	command -v bpftool >/dev/null 2>&1 || { \
		sudo apt-get update && \
		sudo apt-get install -y linux-tools-$$(uname -r) linux-cloud-tools-$$(uname -r) linux-tools-common; \
	}
	command -v clang >/dev/null 2>&1 || { \
		sudo apt-get update && \
		sudo apt-get install -y clang llvm libbpf-dev gcc make; \
	}

kind-install:
	command -v kind >/dev/null 2>&1 || { \
		curl -Lo /tmp/kind https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-linux-amd64; \
		chmod +x /tmp/kind; \
		sudo mv /tmp/kind /usr/local/bin/kind; \
		echo "kind $(KIND_VERSION) instalado"; \
	}

helm-install:
	command -v helm >/dev/null 2>&1 || { \
		curl -Lo /tmp/helm.tar.gz https://get.helm.sh/helm-$(HELM_VERSION)-linux-amd64.tar.gz; \
		tar -xzf /tmp/helm.tar.gz -C /tmp; \
		sudo mv /tmp/linux-amd64/helm /usr/local/bin/helm; \
		rm -rf /tmp/linux-amd64 /tmp/helm.tar.gz; \
		echo "helm $(HELM_VERSION) instalado"; \
	}

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

$(VMLINUX_H):
	@test -r $(VMLINUX_BTF) || { echo "Missing $(VMLINUX_BTF); ensure host BTF is available (CONFIG_DEBUG_INFO_BTF)"; exit 1; }
	@echo "Generating $(VMLINUX_H) from $(VMLINUX_BTF)"
	@bpftool btf dump file $(VMLINUX_BTF) format c > $(VMLINUX_H)

collector-build: bpf-deps $(VMLINUX_H)
	docker build -t collector-ebpf:dev ./collector-ebpf

collector-load:
	kind load docker-image collector-ebpf:dev --name $(CLUSTER_NAME)

collector-up: collector-build collector-load
	kubectl apply -f infra/k8s/collector-ebpf

collector-down:
	kubectl delete -f infra/k8s/collector-ebpf --ignore-not-found
