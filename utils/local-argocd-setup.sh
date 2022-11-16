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

LOCAL_SETUP_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${LOCAL_SETUP_DIR}"/.setupEnv
KCP_GLBC_DIR="${LOCAL_SETUP_DIR}/.."

DO_BREW="false"
NAMESPACE=kcp-glbc

usage() { echo "usage: ./local-argocd-setup.sh -c <number of clusters> <-b>" 1>&2; exit 1; }
while getopts ":bc:" arg; do
  case "${arg}" in
    c)
      NUM_CLUSTERS=${OPTARG}
      ;;
    b)
      DO_BREW="true"
      ;;
    *)
      usage
      ;;
  esac
done
shift $((OPTIND-1))

if [[ "$DO_BREW" == "true" ]]; then
  if [[ "${OSTYPE}" =~ ^darwin.* ]]; then
    ${SCRIPT_DIR}/macos/required_brew_packages.sh
  fi
else
  echo "skipping brew"
fi

source "${LOCAL_SETUP_DIR}"/.startUtils

if [ -z "${NUM_CLUSTERS}" ]; then
    usage
fi

set -e pipefail

TEMP_DIR="./tmp"
KIND_CLUSTER_PREFIX="kcp-cluster-"

: ${GLBC_DEPLOYMENTS_DIR=${KCP_GLBC_DIR}/config/deploy}
: ${KUSTOMIZATION_DIR=${GLBC_DEPLOYMENTS_DIR}/local}
: ${CERT_MANAGER_KUSTOMIZATION_DIR:=${KUSTOMIZATION_DIR}/cert-manager}
: ${ARGOCD_KUSTOMIZATION_DIR:=${GLBC_DEPLOYMENTS_DIR}/../argocd}
SED=sed
if [[ "${OSTYPE}" =~ ^darwin.* ]]; then
  SED=gsed
fi

for ((i=2;i<=$NUM_CLUSTERS;i++))
do
  CLUSTERS="${CLUSTERS}${KIND_CLUSTER_PREFIX}${i} "
done

mkdir -p ${TEMP_DIR}

[[ ! -z "$KUBECONFIG" ]] && KUBECONFIG="$KUBECONFIG" || KUBECONFIG="$HOME/.kube/config"

# cluster, port80, port443, ingress?
createCluster() {
  cluster=$1;
  port80=$2;
  port443=$3;
  ingress=$4;
  cat <<EOF | ${KIND_BIN} create cluster --name ${cluster} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.22.7@sha256:1dfd72d193bf7da64765fd2f2898f78663b9ba366c2aa74be1fd7498a1873166
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=${ingress}"
  extraPortMappings:
  - containerPort: 80
    hostPort: ${port80}
    protocol: TCP
  - containerPort: 443
    hostPort: ${port443}
    protocol: TCP
EOF

  #Note: these aren't used anymore, but leaving for now since the e2e tests still refers to a directory with these kubeconfigs in it
  ${KIND_BIN} get kubeconfig --name=${cluster} > ${TEMP_DIR}/${cluster}.kubeconfig
  ${KIND_BIN} get kubeconfig --internal --name=${cluster} > ${TEMP_DIR}/${cluster}.kubeconfig.internal
}

# cluster, port80, port443, ingress|routes
createKINDCluster() {
  clusterName=${1}
  [[ "$4" == "routes" ]] && ingress="false" || ingress="true"
  echo "Creating KIND Cluster (${clusterName})"
  createCluster $1 $2 $3 ${ingress}

  kubectl config use-context kind-${1}

  if [[ ${ingress} == "true" ]]; then
    echo "Deploying Ingress controller to ${1}"
    VERSION=controller-v1.2.1
    curl https://raw.githubusercontent.com/kubernetes/ingress-nginx/"${VERSION}"/deploy/static/provider/kind/deploy.yaml | sed "s/--publish-status-address=localhost/--report-node-internal-ip-address/g" | kubectl apply -f -
    kubectl annotate ingressclass nginx "ingressclass.kubernetes.io/is-default-class=true"
    echo "Waiting for deployments to be ready ..."
    kubectl -n ingress-nginx wait --timeout=300s --for=condition=Available deployments --all
  else
    kubectl apply -f https://raw.githubusercontent.com/openshift/router/master/deploy/route_crd.yaml
    kubectl apply -f https://raw.githubusercontent.com/openshift/router/master/deploy/router_rbac.yaml
    kubectl create namespace openshift-ingress -o yaml --dry-run=client | kubectl apply -f -
    kubectl apply -f https://raw.githubusercontent.com/openshift/router/master/deploy/router.yaml
    echo "Waiting for deployments to be ready ..."
    kubectl -n openshift-ingress wait --timeout=300s --for=condition=Available deployments --all
  fi
}

#Delete existing kind clusters
clusterCount=$(${KIND_BIN} get clusters | grep ${KIND_CLUSTER_PREFIX} | wc -l)
if ! [[ $clusterCount =~ "0" ]] ; then
  echo "Deleting previous kind clusters."
  ${KIND_BIN} get clusters | grep ${KIND_CLUSTER_PREFIX} | xargs ${KIND_BIN} delete clusters
fi

# (cd ${KCP_GLBC_DIR} && make generate-ld-config)

createKINDCluster "${KIND_CLUSTER_PREFIX}1" 8081 8444 "ingress"

port80=8082
port443=8445
if [ -n "$CLUSTERS" ]; then
  echo "Creating $NUM_CLUSTERS additional cluster(s)"
  for cluster in $CLUSTERS; do
    createKINDCluster "$cluster" $port80 $port443 "ingress"
    port80=$((port80 + 1))
    port443=$((port443 + 1))
  done
fi

echo "Deploying Cert Manager"
kubectl create namespace cert-manager --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/cert-manager.crds.yaml
./bin/kustomize build ${CERT_MANAGER_KUSTOMIZATION_DIR} --enable-helm --helm-command ${HELM_BIN} | kubectl apply -f -
echo "Waiting for Cert Manager deployments to be ready..."
kubectl -n cert-manager wait --timeout=300s --for=condition=Available deployments --all

# Apply the default glbc-ca issuer
kubectl create namespace kcp-glbc --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n kcp-glbc -f ./config/default/issuer.yaml

# Install ArgoCD
wait_for "./bin/kustomize build ${ARGOCD_KUSTOMIZATION_DIR} | kubectl apply -f -" "ArgoCD install, run & CRDs via kustomize" "2m" "20"

wait_for "kubectl -n argocd get secret argocd-initial-admin-secret" "ArgoCD admin secret" "2m" "5"
ARGOCD_ADMIN_PASSWORD=$(kubectl -n argocd get secret argocd-initial-admin-secret -o json | jq ".data.password" -r | base64 --decode)

ports=$(docker ps --format '{{json .}}' | jq 'select(.Names == "kcp-cluster-1-control-plane").Ports')
httpsport=$(echo $ports | sed -e 's/.*0.0.0.0\:\(.*\)->443\/tcp.*/\1/')



echo ""
echo "The kind k8s clusters have been registered, now you should run kcp-glbc."
echo ""
echo "       cd ${PWD}"
echo "       ./bin/kcp-glbc"
echo ""
echo "     ArgoCD: https://argocd.127.0.0.1.nip.io:$httpsport"
echo "     username: admin"
echo "     password: ${ARGOCD_ADMIN_PASSWORD}"
echo ""
echo ""