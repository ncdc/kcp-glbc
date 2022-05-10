# Deployment

## Cert Manager

The GLBC requires cert manager to be deployed to the [glbc control cluster](#glbc-control-cluster-kubeconfig).

```
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/cert-manager.yaml
kubectl -n cert-manager wait --timeout=300s --for=condition=Available deployments --all
```

## GLBC

Deploy the GLBC using a default configuration. 

```
kubectl apply -k github.com/Kuadrant/kcp-glbc/config/default?ref=main
kubectl -n kcp-glbc wait --timeout=300s --for=condition=Available deployments --all
```

The configuration can be modified after the initial deployment as required, see the [configuration](#configuration) section for more details.


## Workload Cluster

When registering a workload cluster with KCP, you need to ensure KCP has permissions to sync the resources you need. To do this you need to create a service account in your workload cluster and create the required cluster role. Then bind the cluster role to the service account. You can then create a kubeconfig and add it to your workload cluster

```
# targeting your workload cluster run:

kubectl create ns kcp-pcluster
kubectl create sa kcp -n kcp-pcluster
kubectl create -f config/kcp/kcp-syncer-role.yaml
# note the script expects the ns to be kcp-pcluster
./utils/workspace_cluster.sh > workload-cluster.yaml

```

## Local Development

The following describes how to deploy and test the GLBC running on a kind cluster created as part of the local development setup.

### Terminal 1

Run the `local-setup` script to create the test kind clusters and start a local KCP process.

```
$ make local-setup
$ kind get clusters
kcp-cluster-1
kcp-cluster-2
kcp-cluster-glbc-control
```

### Terminal 2

Deploy the GLBC to the local glbc control cluster.

```
kubectl config use-context kind-kcp-cluster-glbc-control
make deploy
kubectl -n kcp-glbc wait --timeout=300s --for=condition=Available deployments --all
kubectl logs -f deployments/kcp-glbc-controller-manager -n kcp-glbc
```

If this is the first time running the `make deploy` command it will generate a set of local configuration files which 
can be used to modify all the configuration described below.

```
$ tree -I '*.yaml|*.template' config/deploy/local/
config/deploy/local/
├── aws-credentials.env
├── controller-config.env
└── kcp.kubeconfig
```

These files will not be committed and can be modified as required by you, changes can be applied to the local 
deployment by re-running `make deploy`.

### Terminal 3

Test the deployment using the sample service.

```
export KUBECONFIG=.kcp/admin.kubeconfig
kubectl apply -f samples/echo-service/echo.yaml
```

## Configuration

### KCP Kubeconfig (Required)

A secret `secret/kcp-glbc-kcp-kubeconfig` containing the KCP cluster kubeconfig. An empty secret is created by default 
during installation, but can be replaced with:  

```
kubectl -n kcp-glbc create secret generic kcp-glbc-kcp-kubeconfig --from-file=kubeconfig=$(KCP_KUBECONFIG)
```

### AWS Credentials (Optional) 

A secret  `secret/kcp-glbc-aws-credentials` containing AWS access key and secret. This is only required if `GLBC_DNS_PROVIDER` is set to `aws`.
The credentials must have permissions to create/update/delete records in the hosted zone set in `AWS_DNS_PUBLIC_ZONE_ID`, and the
domain set in `GLBC_DOMAIN` corresponds to the public zone id. An empty secret is created by default during installation, 
but can be replaced with:

```
kubectl -n kcp-glbc create secret generic kcp-glbc-aws-credentials \
--from-literal=AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
--from-literal=AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
```

### GLBC Controller Options (Optional)

A config map `configmap/kcp-glbc-controller-config` containing GLBC configuration options. A config map is created by 
default during installation containing the default values, but can be replaced by editing the config map:

```
kubectl -n kcp-glbc edit configmap kcp-glbc-controller-config
```

| Annotation | Description | Default value |
| ---------- | ----------- | ------------- |
| `AWS_DNS_PUBLIC_ZONE_ID` |  AWS hosted zone id where route53 records will be created (default is dev.hcpapps.net) | Z08652651232L9P84LRSB |
| `GLBC_DNS_PROVIDER` |  The dns provider to use, one of [aws, fake] | fake |
| `GLBC_DOMAIN` |  The domain to use when exposing ingresses via glbc | dev.hcpapps.net |
| `GLBC_ENABLE_CUSTOM_HOSTS` | Allow custom hosts in glbc managed ingresses | false |
| `GLBC_TLS_PROVIDED` | Generate TLS certs for glbc managed hosts | false |
| `GLBC_TLS_PROVIDER` | TLS Cert provider to use, one of [le-staging, le-production] | le-staging |
| `HCG_LE_EMAIL` | EMail address to use during LE cert requests | kuadrant-dev@redhat.com |
| `GLBC_LOGICAL_CLUSTER_TARGET` | logical cluster to target | `*` |

### Applying configuration changes

Any of the described configurations can be modified after the initial creation of the resources, the deploymnet will however 
need to be restarted after each change in order for them to come into affect.

`kubectl rollout restart deployment/kcp-glbc-controller-manager -n kcp-glbc`



## Configuring for remote KCP

If you are not using a local KCP, you will need to create a kubeconfig that allows GLBC to connect to the remote KCP instance. 

To do this, use the following steps. (note you will need the KCP kube plugin which is part of the kcp repo)

1) Login to KCP and select your workspace

```
kubectl kcp workspace my-workspace
```

2) Create a service account

```
kubectl create sa glbc
```

3) create a cluster role and bind it to your service account

```
kubectl create -f config/kcp/glbc-cluster-role.yaml

kubectl create -f config/kcp/glbc-cluster-role-binding.yaml

```

4) extract the service account token

```
kubectl get secrets

$(kubectl --namespace default get secret/glbc-token-<id> -o jsonpath='{.data.token}' | base64 --decode)
```

5) copy the token into a kubeconfig. An example template has been added to ```config/kcp/kcp-cube-config.yaml.template ```


You can now run GLBC targeting the KCP instance by passing this kubeconfig file as a start up parameter

```
-kubeconfig=<path to kcp kube config>

```


