SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

DHALL ?= $(LOCALBIN)/dhall
DHALL_TO_YAML ?= $(LOCALBIN)/dhall-to-yaml
DHALL_TO_JSON ?= $(LOCALBIN)/dhall-to-json
DHALL_TO_YAML_BINARY_LINUX ?= "https://github.com/dhall-lang/dhall-haskell/releases/download/1.41.1/dhall-json-1.7.10-x86_64-linux.tar.bz2"
DHALL_TO_YAML_BINARY_MACOS ?= "https://github.com/dhall-lang/dhall-haskell/releases/download/1.41.1/dhall-json-1.7.10-x86_64-macos.tar.bz2"
DHALL_BINARY_LINUX ?= "https://github.com/dhall-lang/dhall-haskell/releases/download/1.41.1/dhall-1.41.1-x86_64-linux.tar.bz2"
DHALL_BINARY_MACOS ?= "https://github.com/dhall-lang/dhall-haskell/releases/download/1.41.1/dhall-1.41.1-x86_64-macos.tar.bz2"
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
	DHALL_TO_YAML_BINARY=$(DHALL_TO_YAML_BINARY_LINUX)
	DHALL_BINARY=$(DHALL_BINARY_LINUX)
endif
ifeq ($(UNAME_S),Darwin)
	DHALL_TO_YAML_BINARY=$(DHALL_TO_YAML_BINARY_MACOS)
	DHALL_BINARY=$(DHALL_BINARY_MACOS)
endif

.PHONY: dhall
dhall: $(DHALL) ## Download dhall locally if necessary.
$(DHALL):
	curl -L $(DHALL_TO_YAML_BINARY) | tar --extract --bzip2
	curl -L $(DHALL_BINARY) | tar --exclude='./share' --extract --bzip2

DHALL_SOURCE_DIR := config/observability/monitoring_resources
DHALL_SOURCE_FILES=$(shell find $(DHALL_SOURCE_DIR) -type f -name "*.dhall")
DHALL_COMMON_SOURCE_DIR := $(DHALL_SOURCE_DIR)/common
DHALL_RULES_SOURCE_DIR := $(DHALL_COMMON_SOURCE_DIR)/rules
DHALL_K8S_SOURCE_DIR := $(DHALL_SOURCE_DIR)/kubernetes
DHALL_OPENSHIFT_SOURCE_DIR := $(DHALL_SOURCE_DIR)/openshift
DHALL_K8S_TARGET_DIR := config/observability/kubernetes/monitoring_resources
DHALL_OPENSHIFT_TARGET_DIR := config/observability/openshift/monitoring_resources
DHALL_K8S_TARGET_RULES_DIR := $(DHALL_K8S_TARGET_DIR)/rules_unit_tests
DHALL_K8S_TARGETS := $(addprefix $(DHALL_K8S_TARGET_DIR)/,$(patsubst %.dhall,%.yaml,$(shell ls $(DHALL_COMMON_SOURCE_DIR)/*.dhall | xargs -n 1 basename))) $(addprefix $(DHALL_K8S_TARGET_DIR)/,$(patsubst %.dhall,%.yaml,$(shell ls $(DHALL_K8S_SOURCE_DIR)/*.dhall | xargs -n 1 basename)))
DHALL_OPENSHIFT_TARGETS := $(addprefix $(DHALL_OPENSHIFT_TARGET_DIR)/,$(patsubst %.dhall,%.yaml,$(shell ls $(DHALL_COMMON_SOURCE_DIR)/*.dhall | xargs -n 1 basename))) $(addprefix $(DHALL_OPENSHIFT_TARGET_DIR)/,$(patsubst %.dhall,%.yaml,$(shell ls $(DHALL_OPENSHIFT_SOURCE_DIR)/*.dhall | xargs -n 1 basename)))

.PHONY: dhall-format
dhall-format: dhall
	@$(DHALL) lint $(DHALL_SOURCE_FILES)
	@$(DHALL) format $(DHALL_SOURCE_FILES)

define GENERATE_DHALL
	@$(DHALL_TO_YAML) --generated-comment --file $< --output $@
endef

$(DHALL_K8S_TARGET_DIR)/%.yaml: $(DHALL_COMMON_SOURCE_DIR)/%.dhall
	$(GENERATE_DHALL)

$(DHALL_OPENSHIFT_TARGET_DIR)/%.yaml: $(DHALL_COMMON_SOURCE_DIR)/%.dhall
	$(GENERATE_DHALL)

$(DHALL_K8S_TARGET_DIR)/%.yaml: $(DHALL_K8S_SOURCE_DIR)/%.dhall
	$(GENERATE_DHALL)

$(DHALL_OPENSHIFT_TARGET_DIR)/%.yaml: $(DHALL_OPENSHIFT_SOURCE_DIR)/%.dhall
	$(GENERATE_DHALL)

$(DHALL_K8S_TARGETS): | $(DHALL_K8S_TARGET_DIR)

$(DHALL_OPENSHIFT_TARGETS): | $(DHALL_OPENSHIFT_TARGET_DIR)

$(DHALL_K8S_TARGET_DIR):
	mkdir $(DHALL_K8S_TARGET_DIR)

$(DHALL_OPENSHIFT_TARGET_DIR):
	mkdir $(DHALL_OPENSHIFT_TARGET_DIR)

# TODO: detect when dashboard json file has been modified, which is used by dashboard dhall files
# TODO: regen prometheusrule crs when rules files have changed. 
.PHONY: touch-monitoring-files
touch-monitoring-files:
	@find $(DHALL_SOURCE_DIR) -type f -name "*.dhall" -exec touch {} +
	@find $(DHALL_SOURCE_DIR) -type f -name "*.json" -exec touch {} +

# Generate monitoring resources for prometheus etc... 
.PHONY: gen-monitoring-resources
gen-monitoring-resources: touch-monitoring-files gen-rules-list dhall dhall-format ${DHALL_K8S_TARGETS} ${DHALL_OPENSHIFT_TARGETS}

# Ensure the generated monitoring resources are the latest
.PHONY: verify-gen-monitoring-resources
verify-gen-monitoring-resources: gen-monitoring-resources
	git diff --exit-code

.PHONY: gen-new-alert-rule
gen-new-alert-rule: gen-alert-rule-file gen-alert-rule-test gen-alert-rule-runbook gen-monitoring-resources

export RUNBOOK_PATH := https://github.com/Kuadrant/kcp-glbc/blob/main/docs/observability/runbooks/
RUNBOOKS_FOLDER := docs/observability/runbooks
export ALERT_EXPR := vector(1)
.PHONY: gen-alert-rule-file
gen-alert-rule-file:
	@if test -z "$(ALERT_NAME)"; then echo "ALERT_NAME and ALERT_EXPR(optional) are not set. e.g. make gen-new-alert-rule ALERT_NAME=GLBCDown ALERT_EXPR=\"vector(1)\""; exit 1; fi
	@envsubst '$${RUNBOOK_PATH} $${ALERT_NAME} $${ALERT_EXPR}' < $(DHALL_RULES_SOURCE_DIR)/__template__.dhall > $(DHALL_RULES_SOURCE_DIR)/$(ALERT_NAME).dhall
	@echo "Generated rule source file:"
	@echo "  $(DHALL_RULES_SOURCE_DIR)/$(ALERT_NAME).dhall"
	@echo "Modify the alert rule expression and other fields."
	@echo "After any changes you need to regerate the various rules yaml files."
	@echo "Run: 'make gen-monitoring-resources' and check in the generated files."
	@echo

.PHONY: gen-alert-rule-test
gen-alert-rule-test:
	@if test -z "$(ALERT_NAME)"; then echo "ALERT_NAME is not set. e.g. make gen-alert-rule-test ALERT_NAME=GLBCDown"; exit 1; fi
	@envsubst '$${RUNBOOK_PATH} $${ALERT_NAME} $${ALERT_EXPR}' < $(DHALL_K8S_TARGET_RULES_DIR)/test.yaml.template > $(DHALL_K8S_TARGET_RULES_DIR)/$(ALERT_NAME)_test.yaml
	@echo "Generated rule unit test file:"
	@echo "  $(DHALL_K8S_TARGET_RULES_DIR)/$(ALERT_NAME)_test.yaml"
	@echo "Modify and extend the skeleton test case."
	@echo "See https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/ for promtool unit test case syntax."
	@echo "To execute unit tests, run: 'make prometheus-rules-unit-test'"
	@echo

.PHONY: gen-alert-rule-runbook
gen-alert-rule-runbook:
	@if test -z "$(ALERT_NAME)"; then echo "ALERT_NAME is not set. e.g. make gen-alert-rule-runbook ALERT_NAME=GLBCDown"; exit 1; fi
	@envsubst '$${ALERT_NAME}' < $(RUNBOOKS_FOLDER)/__template__.adoc > $(RUNBOOKS_FOLDER)/$(ALERT_NAME).adoc
	@echo "Generated rule runbook file:"
	@echo "  $(RUNBOOKS_FOLDER)/$(ALERT_NAME).adoc"
	@echo "Add useful commands and troubleshooting steps to this runbook for how to fix the cause of the firing alert."
	@echo

.PHONY: gen-rules-list
gen-rules-list: RULES_FILES="$(shell echo $(addprefix "./", $(shell ls $(DHALL_RULES_SOURCE_DIR)|grep -v -E '__.*__.dhall|DeadMansSwitch.dhall|SLOIngressAdmissionLatency.dhall')) | sed 's/ /, /g')"
gen-rules-list:
	@echo "let rules = [ $(RULES_FILES) ] in rules" > $(DHALL_RULES_SOURCE_DIR)/__glbc__.dhall

# Run all prometheus (alert) rules unit tests
.PHONY: prometheus-rules-unit-test
prometheus-rules-unit-test:
	docker run --rm -t \
    -v $(shell pwd)/$(DHALL_K8S_TARGET_DIR):/prometheus:z --entrypoint=/bin/sh \
$(PROMTOOL_IMAGE) -c 'promtool test rules /prometheus/rules_unit_tests/*.yaml'
	docker run --rm -t \
    -v $(shell pwd)/$(DHALL_OPENSHIFT_TARGET_DIR):/prometheus:z --entrypoint=/bin/sh \
$(PROMTOOL_IMAGE) -c 'promtool test rules /prometheus/rules_unit_tests/*.yaml'
