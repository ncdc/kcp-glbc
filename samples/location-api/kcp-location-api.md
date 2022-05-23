

KCP Related PRs and issues:

* Demo script issue https://github.com/kcp-dev/kcp/issues/912
* Anything related to location API (Scheduling) on 0.4.0 was added as part of https://github.com/kcp-dev/kcp/pull/829
* Advanced scheduling (main/0.5.0) https://github.com/kcp-dev/kcp/pull/942
* LocationAPI NS MVP https://github.com/kcp-dev/kcp/pull/1084


## KCP 0.4.0

Location APIs are alpha status and disabled by default, you can enable on kcp startup with:

```
start --discovery-poll-interval 3s --run-controllers --feature-gates=KCPLocationAPI=true
```

```
$ kubectl api-resources | grep kcp
apiresourceimports                             apiresource.kcp.dev/v1alpha1           false        APIResourceImport
negotiatedapiresources                         apiresource.kcp.dev/v1alpha1           false        NegotiatedAPIResource
apibindings                                    apis.kcp.dev/v1alpha1                  false        APIBinding
apiexports                                     apis.kcp.dev/v1alpha1                  false        APIExport
apiresourceschemas                             apis.kcp.dev/v1alpha1                  false        APIResourceSchema
locations                                      scheduling.kcp.dev/v1alpha1            false        Location
workloadclusters                               workload.kcp.dev/v1alpha1              false        WorkloadCluster
```

== Start KCP == 

```
make local-setup
```

Test based on github.com/kcp-dev/kcp/test/e2e/reconciler/scheduling/controller_test.go

== Negotiation WorkSpace (kcp-glbc)

1. Check ApiResourceImports

```
$ kubectl get apiresourceimports
NAME
deployments.kcp-cluster-1.v1.apps
deployments.kcp-cluster-2.v1.apps
services.kcp-cluster-1.v1.core
services.kcp-cluster-2.v1.core
```

2. Check NegotiatedAPIResources

```
$ kubectl get negotiatedapiresource
NAME
deployments.v1.apps
services.v1.core
```

3. Create kubernetes ApiExport
```
$ kubectl apply -f samples/location-api/kubernetes-api-export.yaml
apiexport.apis.kcp.dev/kubernetes created
$ kubectl get apiexports
NAME         AGE
kubernetes   13s
```

4. Check ApiResourceSchemas
```
$ kubectl get apiresourceschemas
NAME                       AGE
rev-248.services.core      79s
rev-258.deployments.apps   79s
```

5. Create Location
```
$ kubectl apply -f samples/location-api/location.yaml
location.scheduling.kcp.dev/us-east1 created
$ kubectl get locations
NAME       RESOURCE           AVAILABLE   INSTANCES   LABELS
us-east1   workloadclusters   2           2 
```

== User WorkSpace (tstuserws)

```
$ ./bin/kubectl-kcp workspace use root:default
Current workspace is "root:default".
$ ./bin/kubectl-kcp workspace create tstuserws --enter
Workspace "tstuserws" (type "Universal") created. Waiting for being ready.
Current workspace is "root:default:tstuserws".
```

6. Create API Binding 
```
$ kubectl apply -f samples/location-api/kubernetes-api-binding.yaml
apibinding.apis.kcp.dev/kubernetes created
$ kubectl get apibindings
NAME         AGE
kubernetes   14
```

7. Check Services
```
$ kubectl get services
No resources found in default namespace.

$ kubectl get ns default -o json | jq .metadata.annotations
{
"scheduling.kcp.dev/placement": "{\"us-east1+f6a7a93b-9c61-42a1-8a90-eeaa5dce6ff5\":\"Pending\"}"
}
```

Status:

* Location never changes from pending
* Nothing hapopens when you deploy the echo service inside the user ns, no scheduling appears to exist as yet.
* Looks as though the scheduling is not planned unti l 0.5.0, and so location APIs wil not be usuable until then
* PR here to enable LocationAPIs and implements a location API MVP https://github.com/kcp-dev/kcp/pull/1084

## KCP https://github.com/kcp-dev/kcp/pull/1084

== Negotiation WorkSpace (kcp-glbc)

1. Check ApiResourceImports
```
$ kubectl get apiresourceimports
NAME
deployments.kcp-cluster-1.v1.apps
deployments.kcp-cluster-2.v1.apps
services.kcp-cluster-1.v1.core
services.kcp-cluster-2.v1.core
```

2. Check NegotiatedAPIResources
```
$ kubectl get negotiatedapiresource
NAME
deployments.v1.apps
services.v1.core
```

3. Check kubernetes ApiExport
```
$ kubectl get apiexports
NAME         AGE
kubernetes   19m
```

4. Check ApiResourceSchemas
```
$ kubectl get apiresourceschemas
NAME                       AGE
rev-250.services.core      19m
rev-253.deployments.apps   19m
```

5. Create Location
```
$ kubectl apply -f samples/location-api/location.yaml
location.scheduling.kcp.dev/us-east1 created
$ kubectl get locations
NAME       RESOURCE           AVAILABLE   INSTANCES   LABELS
us-east1   workloadclusters   2           2
```

== User WorkSpace (tstuserws)
```
$ ./bin/kubectl-kcp workspace use root:default
Current workspace is "root:default".
$ ./bin/kubectl-kcp workspace create tstuserws --enter
Workspace "tstuserws" (type "Universal") created. Waiting for being ready.
Current workspace is "root:default:tstuserws".
```

6. Create API Binding
```
$ kubectl apply -f samples/location-api/kubernetes-api-binding.yaml
apibinding.apis.kcp.dev/kubernetes created
$ kubectl get apibindings
NAME         AGE
kubernetes   14
```

7. Check Services
```
$ kubectl get services
No resources found in default namespace.

$ kubectl get ns default -o json | jq .metadata.annotations
{
"internal.scheduling.kcp.dev/negotiation-workspace": "root:default:kcp-glbc",
"scheduling.kcp.dev/placement": "{\"root:default:kcp-glbc+us-east1+kcp-cluster-2\":\"Pending\"}"
}
```
