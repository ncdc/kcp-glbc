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

DEPLOY_SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
KCP_GLBC_DIR="${DEPLOY_SCRIPT_DIR}/.."
source "${DEPLOY_SCRIPT_DIR}"/.setupEnv
source "${DEPLOY_SCRIPT_DIR}"/.startUtils

OUTPUT_DIR=${KCP_GLBC_DIR}/tmp
CA_DATA=""

NAMESPACE=kcp-glbc
SERVICE_ACCOUNT_NAME=kcp-glbc-controller-manager

trap cleanup EXIT 1 2 3 6 15

cleanup() {
  git checkout ${KCP_GLBC_DIR}/config/kcp/kustomization.yaml
}

############################################################
# Help                                                     #
############################################################
help()
{
   # Display Help
   echo "Create the kcp-glbc namespace, kcp-glbc-controller-manager service account and generates a kubeconfig for access."
   echo
   echo "Syntax: create_glbc_kubeconfig.sh [-n|c|C|o]"
   echo "options:"
   echo "a     Certificate Authority Data to add to the cluster of the generated kubeconfig (default: ${CA_DATA})."
   echo "h     Print this Help."
   echo "n     Namespace (default: ${NAMESPACE})"
   echo "o     Output directory where generated kubeconfig will be written (default: ${OUTPUT_DIR}/<namespace>-<cluster name>.kubeconfig)."
   echo
}

while getopts "a:n:o:" arg; do
  case "${arg}" in
    a)
      CA_DATA=${OPTARG}
      ;;
    n)
      NAMESPACE=${OPTARG}
      ;;
    o)
      OUTPUT_FILE=${OPTARG}
      ;;
    *)
      help
      exit 0
      ;;
  esac
done
shift $((OPTIND-1))

set -e pipefail

if [ -z "$OUTPUT_FILE" ]; then
  OUTPUT_FILE=${OUTPUT_DIR}/"kcp.kubeconfig"
fi

cd config/kcp/ || exit
../../bin/kustomize edit set namespace $NAMESPACE
cd ../..
./bin/kustomize build ${KCP_GLBC_DIR}/config/kcp | kubectl apply -f -

# Generate kubeconfig
secretName=$(kubectl get sa "$SERVICE_ACCOUNT_NAME" --namespace="$NAMESPACE" -o json | jq -r .secrets[0].name)
echo "secretName: ${secretName}"
secretToken=$(kubectl get secret --namespace "$NAMESPACE" "${secretName}" -o json | jq -r '.data["token"]' | base64 --decode)
echo "secretToken: ${secretToken}"
currentContext=$(kubectl config current-context)
echo "currentContext: ${currentContext}"
currentCluster=$(kubectl config get-contexts "$currentContext" | awk '{print $3}' | tail -n 1)
echo "currentCluster: ${currentCluster}"
clusterServer=$(kubectl config view -o jsonpath="{.clusters[?(@.name == \"${currentCluster}\")].cluster.server}")
echo "clusterServer: ${clusterServer}"
clusterServer=$(echo $clusterServer | cut -d'/' -f1,2,3)
echo "clusterServer: ${clusterServer}"

echo "apiVersion: v1
kind: Config
clusters:
  - name: glbc
    cluster:
      server: ${clusterServer}
contexts:
  - name: glbc@glbc
    context:
      cluster: glbc
      namespace: $NAMESPACE
      user: glbc
users:
  - name: glbc
    user:
      token: ${secretToken}
current-context: glbc@glbc" > ${OUTPUT_FILE}

## Get the ca data for this KCP if it exists, used later to inject into generated kubeconfigs
caData=$(kubectl config view --raw -o json | jq -r '.clusters[0].cluster."certificate-authority-data"' | tr -d '"')
echo ${caData}

#ToDO Check the contents of caData are valid and not "null"
if [  ! -z "${caData}" ] ; then
  echo "${caData}" | base64 --decode > "${KCP_GLBC_DIR}/tmp/ca.crt"
  kubectl config set-cluster glbc \
  --kubeconfig="${OUTPUT_FILE}" \
  --server="${clusterServer}" \
  --certificate-authority="${KCP_GLBC_DIR}/tmp/ca.crt" \
  --embed-certs=true
fi

echo ""
echo "KUBECONFIG: ${OUTPUT_FILE}"
echo ""
echo "Test with: kubectl --kubeconfig ${OUTPUT_FILE} api-resources"
