#!/bin/bash

#
# Copyright 2022 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Deploys GLBC and the GLBC monitoring stack (prometheus/grafana) onto the kcp-cluster-glbc-control kind cluster.
# You MUST run local-setup before running this script.

set -e pipefail

wait_for() {
  local command="${1}"
  local description="${2}"
  local timeout="${3}"
  local interval="${4}"

  printf "Waiting for %s for %s...\n" "${description}" "${timeout}"
  timeout --foreground "${timeout}" bash -c "
    until ${command}
    do
        printf \"Waiting for %s... Trying again in ${interval}s\n\" \"${description}\"
        sleep ${interval}
    done
    "
  printf "%s finished!\n" "${description}"
}

#Deploy kcp-glbc
kubectl config use-context kind-kcp-cluster-glbc-control
make deploy
kubectl -n kcp-glbc wait --timeout=300s --for=condition=Available deployments --all

#Deploy Monitoring stack (Prometheus and Grafana)
wait_for "./bin/kustomize build config/observability/kubernetes/ | kubectl apply --force-conflicts  --server-side -f -" "monitoring stack" "1m" "5"
# Requires a second run to ensure everything is created
# unable to recognize "STDIN": no matches for kind "Grafana" in version "integreatly.org/v1alpha1"
# unable to recognize "STDIN": no matches for kind "GrafanaDataSource" in version "integreatly.org/v1alpha1"
# unable to recognize "STDIN": no matches for kind "Alertmanager" in version "monitoring.coreos.com/v1"
# etc..

#Wait for prometheus
kubectl -n monitoring wait --timeout=300s --for=condition=Available deployments --all
#Wait for grafana
kubectl -n kcp-glbc-observability wait --timeout=300s --for=condition=Available deployments --all

#Deploy Pod Monitor for kcp-glbc
./bin/kustomize build config/prometheus/ | kubectl -n kcp-glbc apply -f -

#Check kcp-glbc Prometheus
wait_for "kubectl -n monitoring get secret prometheus-k8s -o json | jq -r '.data[\"prometheus.yaml.gz\"]'| base64 -d | gunzip | grep kcp-glbc" "kcp-glbc prometheus config" "1m" "10"

ports=$(docker ps --format '{{json .}}' | jq 'select(.Names == "kcp-cluster-glbc-control-control-plane").Ports')
httpport=$(echo $ports | sed -e 's/.*0.0.0.0\:\(.*\)->80\/tcp.*/\1/')

wait_for "curl -s http://prometheus.127.0.0.1.nip.io:$httpport/api/v1/targets | jq -e '.data.activeTargets[] | select(.labels.job == \"kcp-glbc/kcp-glbc-controller-manager\").labels'" "kcp-glbc prometheus target" "1m" "10"

echo ""
echo "GLBC is now running on the kcp-cluster-glbc-control kind cluster with monitoring enabled."
echo ""
echo "Tail the logs:"
echo ""
echo "     kubectl --context kind-kcp-cluster-glbc-control logs -f deployments/kcp-glbc-controller-manager -n kcp-glbc"
echo ""
echo "Monitoring consoles:"
echo ""
echo "     Prometheus: http://prometheus.127.0.0.1.nip.io:$httpport"
echo "     Grafana:    http://grafana.127.0.0.1.nip.io:$httpport     (user:admin, password: admin)"
echo ""
