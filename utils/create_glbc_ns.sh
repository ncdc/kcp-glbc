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

# Creates a namespace in the current kube context and outputs a kubeconfig which has permissions to access all
# resources relevant to GLBC in that namespace.
#
# ./bin/kubectl-kcp workspace create my-glbc --enter
# ./utils/create_glbc_ns.sh -n bob -c my-glbc
# kubectl --kubeconfig tmp/bob-my-glbc.kubeconfig api-resources
#

trap cleanup EXIT 1 2 3 6 15

cleanup() {
  git checkout config/kcp/kustomization.yaml
}

usage() { echo "usage: ./create_glbc_ns.sh -n <namespace> -c <cluster name>" 1>&2; exit 1; }

while getopts "n:c:" arg; do
  case "${arg}" in
    n)
      # The namespace to create
      namespace=${OPTARG}
      ;;
    c)
      # Unique name for the cluster, can be anything
      clusterName=${OPTARG}
      ;;
    *)
      usage
      ;;
  esac
done
shift $((OPTIND-1))

if [ -z "$namespace" ] || [ -z "$clusterName" ]; then
  usage
fi

## Create ns/role/rolebinding and serviceAccount for user
cd config/kcp/ || exit
../../bin/kustomize edit set namespace $namespace
cd ../..
./bin/kustomize build config/kcp | kubectl apply -f -

# Generate kubeconfig
serviceAccount=glbc
secretName=$(kubectl get sa "$serviceAccount" --namespace="$namespace" -o json | jq -r .secrets[0].name)
echo "secretName: ${secretName}"
secretToken=$(kubectl get secret --namespace "${namespace}" "${secretName}" -o json | jq -r '.data["token"]' | base64 --decode)
echo "secretToken: ${secretToken}"
currentContext=$(kubectl config current-context)
echo "currentContext: ${currentContext}"
currentCluster=$(kubectl config get-contexts "$currentContext" | awk '{print $3}' | tail -n 1)
echo "currentCluster: ${currentCluster}"
clusterServer=$(kubectl config view -o jsonpath="{.clusters[?(@.name == \"${currentCluster}\")].cluster.server}")
echo "clusterServer: ${clusterServer}"

echo "apiVersion: v1
kind: Config
clusters:
  - name: ${clusterName}
    cluster:
      server: ${clusterServer}
contexts:
  - name: ${serviceAccount}@${clusterName}
    context:
      cluster: ${clusterName}
      namespace: ${namespace}
      user: ${serviceAccount}
users:
  - name: ${serviceAccount}
    user:
      token: ${secretToken}
current-context: ${serviceAccount}@${clusterName}" > tmp/"${namespace}-${clusterName}.kubeconfig"

echo ""
echo "KUBECONFIG: tmp/${namespace}-${clusterName}.kubeconfig"
echo ""
echo "Test with: kubectl --kubeconfig tmp/${namespace}-${clusterName}.kubeconfig api-resources"
