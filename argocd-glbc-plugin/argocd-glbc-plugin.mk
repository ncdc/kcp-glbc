SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
K8S_VERSION ?= 1.23.12

argocd-plugin-bin:
	mkdir -p  $(SELF_DIR)argocd-cmp-plugin/bin

KIND_ADMIN_KUBECONFIG ?= $(SELF_DIR)/kubeconfig
argocd-start: kind argocd-cmp-plugin
	KUBECONFIG=$(KIND_ADMIN_KUBECONFIG) $(KIND) create cluster --wait 5m --config $(SELF_DIR)kind.yaml --image kindest/node:v${K8S_VERSION}
	@make -s argocd-setup

argocd-stop:
	kind delete cluster --name=kind || true

argocd-clean:
	rm -rf $(SELF_DIR)kubeconfig $(SELF_DIR)argocd-cmp-plugin/bin

CMP_PLUGIN ?= $(SELF_DIR)argocd-cmp-plugin/bin/argocd-glbc-plugin
argocd-cmp-plugin: $(CMP_PLUGIN)
$(CMP_PLUGIN): argocd-plugin-bin $(SELF_DIR)plugin.go
	cd $(SELF_DIR) && GOOS=linux CGO_ENABLED=0 go build -o argocd-cmp-plugin/bin/argocd-glbc-plugin plugin.go
	chmod +x $(CMP_PLUGIN)

ARGOCD_PASSWD = $(shell kubectl --kubeconfig=$(KIND_ADMIN_KUBECONFIG) -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
argocd-password:
	@echo $(ARGOCD_PASSWD)

argocd-setup: export KUBECONFIG=$(KIND_ADMIN_KUBECONFIG)
argocd-setup: kustomize
	$(KUSTOMIZE) build $(SELF_DIR)argocd/argocd-install | kubectl apply -f -
	kubectl -n argocd wait deployment argocd-server --for condition=Available=True --timeout=90s
	kubectl port-forward svc/argocd-server -n argocd 8080:80 > /dev/null  2>&1 &
	@echo -ne "\n\n\tConnect to ArgoCD UI in https://localhost:8080\n\n"
	@echo -ne "\t\tUser: admin\n"
	@echo -ne "\t\tPassword: "
	@make -s argocd-password
	@echo

argocd-port-forward-stop:
	pkill kubectl

argocd-example-glbc-application: export KUBECONFIG=$(KIND_ADMIN_KUBECONFIG)
argocd-example-glbc-application:
	kubectl -n argocd apply -f $(SELF_DIR)argocd/argocd-install/application-glbc-example.yaml

argocd-cmp-logs: export KUBECONFIG=$(KIND_ADMIN_KUBECONFIG)
argocd-cmp-logs:
	kubectl -n argocd logs -fl "app.kubernetes.io/name=argocd-repo-server" -c plugin
