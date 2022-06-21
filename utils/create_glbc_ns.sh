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

DEPLOY_SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
KCP_GLBC_DIR="${DEPLOY_SCRIPT_DIR}/.."
source "${DEPLOY_SCRIPT_DIR}"/.setupEnv
source "${DEPLOY_SCRIPT_DIR}"/.startUtils

CLUSTER_ROLE=false
OUTPUT_DIR=${KCP_GLBC_DIR}/tmp
CA_DATA=""

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
   echo "Creates a namespace in the current context and outputs a kubeconfig which has permissions"
   echo "to access all resources relevant to GLBC in that namespace."
   echo
   echo "Syntax: create_glbc_ns.sh [-n|c|C|o]"
   echo "options:"
   echo "a     Certificate Authority Data to add to the cluster of the generated kubeconfig (default: ${CA_DATA})."
   echo "c     Cluster Name (required)"
   echo "C     Apply cluster role permissions (default: ${CLUSTER_ROLE})."
   echo "h     Print this Help."
   echo "n     Namespace (required)"
   echo "o     Output directory where generated kubeconfig will be written (default: ${OUTPUT_DIR}/<namesapce>-<cluster name>.kubeconfig)."
   echo
}

while getopts "a:n:c:Co:" arg; do
  case "${arg}" in
    a)
      CA_DATA=${OPTARG}
      ;;
    n)
      # The namespace to create
      namespace=${OPTARG}
      ;;
    c)
      # Unique name for the cluster, can be anything
      clusterName=${OPTARG}
      ;;
    C)
      CLUSTER_ROLE=true
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

if [ -z "$namespace" ] || [ -z "$clusterName" ]; then
  help
  exit 1
fi

if [ -z "$OUTPUT_FILE" ]; then
  OUTPUT_FILE=${OUTPUT_DIR}/"${namespace}-${clusterName}.kubeconfig"
fi

serviceAccount=glbc

if [ "$CLUSTER_ROLE" = true ] ; then
  namespace="default"
  kubectl create sa $serviceAccount
  kubectl apply -f ${KCP_GLBC_DIR}/config/kcp/glbc-cluster-role.yaml
  kubectl apply -f ${KCP_GLBC_DIR}/config/kcp/glbc-cluster-role-binding.yaml
else
  kubectl delete -f ${KCP_GLBC_DIR}/config/kcp/glbc-cluster-role.yaml
  kubectl delete -f ${KCP_GLBC_DIR}/config/kcp/glbc-cluster-role-binding.yaml
  ## Create ns/role/rolebinding and serviceAccount for user
  cd config/kcp/ || exit
  ../../bin/kustomize edit set namespace $namespace
  cd ../..
  ./bin/kustomize build ${KCP_GLBC_DIR}/config/kcp | kubectl apply -f -
fi

# Generate kubeconfig
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

if [ "$CLUSTER_ROLE" = true ] ; then
  clusterServer=$(echo $clusterServer | cut -d'/' -f1,2,3)
fi

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
current-context: ${serviceAccount}@${clusterName}" > ${OUTPUT_FILE}

echo ${CA_DATA}

#ToDO Check the contents of CA_DATA are valid and not "null"
if [  ! -z "${CA_DATA}" ] ; then
  echo "${CA_DATA}" | base64 --decode > "${KCP_GLBC_DIR}/tmp/ca.crt"
  kubectl config set-cluster "${clusterName}" \
  --kubeconfig="${OUTPUT_FILE}" \
  --server="${clusterServer}" \
  --certificate-authority="${KCP_GLBC_DIR}/tmp/ca.crt" \
  --embed-certs=true
fi

echo ""
echo "KUBECONFIG: ${OUTPUT_FILE}"
echo ""
echo "Test with: kubectl --kubeconfig ${OUTPUT_FILE} api-resources"
