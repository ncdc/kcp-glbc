SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
K8S_VERSION ?= 1.23.12
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m | sed 's/x86_64/amd64/')

KFILT = docker run --rm -i ryane/kfilt

argocd-plugin-bin:
	mkdir -p  $(SELF_DIR)bin

GLBC_KUBECONFIG ?= $(CLUSTERS_KUBECONFIG_DIR)/kcp-cluster-1.kubeconfig
argocd-glbc-start: docker-build
	$(MAKE) local-setup NUM_CLUSTERS=1 LOCAL_SETUP_BACKGROUND=1 || true
	$(KIND) load docker-image ${IMG} --name kcp-cluster-1
	KUBECONFIG=$(KUBECONFIG) ./utils/deploy.sh -k config/deploy/local

argocd-glbc-stop:
	pkill kcp
	$(KIND) delete cluster --name=kcp-cluster-1 || true

ARGOCD_KUBECONFIG ?= $(SELF_DIR)/kubeconfig
argocd-start: kind argocd-build-plugin
	KUBECONFIG=$(ARGOCD_KUBECONFIG) $(KIND) create cluster --name argocd --wait 5m --config $(SELF_DIR)kind.yaml --image kindest/node:v${K8S_VERSION}
	@make -s argocd-setup

argocd-stop:
	$(KIND) delete cluster --name=argocd || true

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
ARGOCD_TOKEN = $(shell $(MAKE) -s argocd-login && $(ARGOCD) proj role create-token example-glbc-project admin --token-only)
argocd-plugin-config: argocd
	ARGOCD_TOKEN=$(ARGOCD_TOKEN) GLBC_HOST=$(GLBC_HOST) GLBC_IP=$(GLBC_IP) \
		envsubst < $(PLUGIN_CONFIG).template > $(PLUGIN_CONFIG)

ARGOCD_PASSWD = $(shell kubectl --kubeconfig=$(ARGOCD_KUBECONFIG) -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
argocd-password:
	@echo $(ARGOCD_PASSWD)

argocd-login: argocd
	@$(ARGOCD) login localhost:8443 --insecure --username admin --password $(ARGOCD_PASSWD) > /dev/null

argocd-setup: export KUBECONFIG=$(ARGOCD_KUBECONFIG)
argocd-setup: kustomize
	$(KUSTOMIZE) build $(SELF_DIR)config/ | $(KFILT) -i kind=CustomResourceDefinition | kubectl apply -f -
	$(KUSTOMIZE) build $(SELF_DIR)config/ | kubectl apply -f -
	kubectl -n argocd wait deployment argocd-server --for condition=Available=True --timeout=90s
	kubectl port-forward svc/argocd-server -n argocd 8443:443 > /dev/null  2>&1 &
	$(MAKE) -s argocd-plugin-config
	$(KUSTOMIZE) build $(SELF_DIR)config/ | $(KFILT) -i kind=ConfigMap,name=cmp-plugin  | kubectl apply -f -
	$(MAKE) -s argocd-refresh-repo-server
	@echo -ne "\n\n\tConnect to ArgoCD UI in https://localhost:8443\n\n"
	@echo -ne "\t\tUser: admin\n"
	@echo -ne "\t\tPassword: "
	@make -s argocd-password
	@echo

argocd-port-forward-stop:
	pkill kubectl

argocd-cmp-logs: export KUBECONFIG=$(ARGOCD_KUBECONFIG)
argocd-cmp-logs:
	kubectl -n argocd logs -fl "app.kubernetes.io/name=argocd-repo-server" -c plugin

GLBC_POD_NAMESPACE = $(shell kubectl --kubeconfig=$(GLBC_KUBECONFIG) get pods -A | awk '/kcp-glbc-controller-manager/{print $$1}')
argocd-glbc-logs:
	kubectl --kubeconfig=$(GLBC_KUBECONFIG) -n $(GLBC_POD_NAMESPACE) logs -f -l app.kubernetes.io/name=kcp-glbc

argocd-refresh-repo-server: export KUBECONFIG=$(ARGOCD_KUBECONFIG)
argocd-refresh-repo-server:
	kubectl -n argocd delete pod -l app.kubernetes.io/name=argocd-repo-server --force --grace-period=0

##@ Install argocd and configure the root:test workspace
ARGOCD ?= $(LOCALBIN)/argocd
ARGOCD_VERSION ?= v2.4.12
ARGOCD_DOWNLOAD_URL ?= https://github.com/argoproj/argo-cd/releases/download/v2.4.13/argocd-$(OS)-$(ARCH)
argocd: $(ARGOCD) ## Download argocd CLI locally if necessary
$(ARGOCD):
	curl -sL $(ARGOCD_DOWNLOAD_URL) -o $(ARGOCD)
	chmod +x $(ARGOCD)
