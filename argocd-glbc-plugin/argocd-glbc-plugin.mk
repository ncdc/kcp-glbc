SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
K8S_VERSION ?= 1.23.12

KFILT = docker run --rm -i ryane/kfilt

argocd-plugin-bin:
	mkdir -p  $(SELF_DIR)bin

GLBC_KUBECONFIG ?= $(CLUSTERS_KUBECONFIG_DIR)/kcp-cluster-1.kubeconfig
argocd-glbc-start:
	$(MAKE) local-setup NUM_CLUSTERS=1 LOCAL_SETUP_BACKGROUND=1 || true
	$(KIND) load docker-image ${IMG} --name kcp-cluster-1
	KUBECONFIG=$(KUBECONFIG) ./utils/deploy.sh -k config/deploy/local

argocd-glbc-stop:
	pkill kcp
	$(KIND) delete cluster --name=kcp-cluster-1 || true

ARGOCD_KUBECONFIG ?= $(SELF_DIR)/kubeconfig
argocd-start: kind argocd-build-plugin
	KUBECONFIG=$(ARGOCD_KUBECONFIG) $(KIND) create cluster --wait 5m --config $(SELF_DIR)kind.yaml --image kindest/node:v${K8S_VERSION}
	$(KIND) load docker-image quay.io/kuadrant/kcp-glbc:latest --name kind
	@make -s argocd-setup

argocd-stop:
	$(KIND) delete cluster --name=kind || true

argocd-clean:
	rm -rf $(SELF_DIR)kubeconfig $(SELF_DIR)bin

CMP_PLUGIN ?= $(SELF_DIR)bin/argocd-glbc-plugin
argocd-build-plugin: $(CMP_PLUGIN)
$(CMP_PLUGIN): argocd-plugin-bin $(SELF_DIR)plugin.go
	cd $(SELF_DIR) && GOOS=linux CGO_ENABLED=0 go build -o bin/argocd-glbc-plugin plugin.go
	chmod +x $(CMP_PLUGIN)

PLUGIN_CONFIG ?= $(SELF_DIR)config/argocd-install/plugin-config.yaml
GLBC_HOST = $(shell kubectl --kubeconfig $(GLBC_KUBECONFIG) get ingress -A | awk '/ingress-glbc-transform/{print $$4}')
GLBC_IP = $(shell kubectl --kubeconfig $(GLBC_KUBECONFIG) get ingress -A | awk '/ingress-glbc-transform/{print $$5}')
argocd-plugin-config:
	GLBC_HOST=$(GLBC_HOST) GLBC_IP=$(GLBC_IP) envsubst < $(PLUGIN_CONFIG).template > $(PLUGIN_CONFIG)

ARGOCD_PASSWD = $(shell kubectl --kubeconfig=$(ARGOCD_KUBECONFIG) -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
argocd-password:
	@echo $(ARGOCD_PASSWD)

argocd-setup: export KUBECONFIG=$(ARGOCD_KUBECONFIG)
argocd-setup: kustomize argocd-plugin-config
	$(KUSTOMIZE) build $(SELF_DIR)config/ | kubectl apply -f -
	kubectl -n argocd wait deployment argocd-server --for condition=Available=True --timeout=90s
	kubectl port-forward svc/argocd-server -n argocd 8080:80 > /dev/null  2>&1 &
	@echo -ne "\n\n\tConnect to ArgoCD UI in https://localhost:8080\n\n"
	@echo -ne "\t\tUser: admin\n"
	@echo -ne "\t\tPassword: "
	@make -s argocd-password
	@echo

argocd-port-forward-stop:
	pkill kubectl

argocd-example-glbc-application: export KUBECONFIG=$(ARGOCD_KUBECONFIG)
argocd-example-glbc-application:
	kubectl -n argocd apply -f $(SELF_DIR)config/argocd-install/application-glbc-example.yaml

argocd-cmp-logs: export KUBECONFIG=$(ARGOCD_KUBECONFIG)
argocd-cmp-logs:
	kubectl -n argocd logs -fl "app.kubernetes.io/name=argocd-repo-server" -c plugin
