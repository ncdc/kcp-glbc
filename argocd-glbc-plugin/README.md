# ArgoCD plugin dev environment

## Setup

Start GLBC within kcp. Currently is not possible to run glbc out of kcp.

`make argocd-glbc-start`

Start a kind cluster and install argocd in it. ArgoCD UI is accessible at https://localhost:8080

`make argocd-start`

Show the logs of the plugin. It might take a moment for the pod to start.

`make argocd-cmp-logs`

Show the logs for the glbc-controller

`make argocd-glbc-logs`

Create an ArgoCD Application pointing to this repo

`make argocd-example-glbc-application`

Sync the Application from the ArgoCD UI.
If it fails to sync, it may be because:

* the argocd-repo-server or some other argo component is still initializing.
* there is a problem with the argocd-cmp-server sidecar
* there is a problem with the argocd-glbc-lugin binary

Either way, check the argocd-cmp-server logs.
If syncing was successful, you should see something like the below:

```
time="2022-11-22T21:04:22Z" level=info msg="Generating manifests with no request-level timeout"
time="2022-11-22T21:04:22Z" level=info msg="argocd-glbc-plugin generate --url http://cdujb5fkflvd7skqupmg.dev.hcpapps.net/transform --resolve 172.18.0.3:80" dir=/tmp/_cmp_server/e9869c8c-887e-4df5-84e0-8a33b64f03f9/argocd-glbc-plugin/manifests/test-app execID=1834a
time="2022-11-22T21:04:22Z" level=info msg="finished streaming call with code OK" grpc.code=OK grpc.method=GenerateManifest grpc.service=plugin.ConfigManagementPluginService grpc.start_time="2022-11-22T21:04:21Z" grpc.time_ms=468.663 span.kind=server system=grpc
```

In the glbc-controller logs you should see the request from the plugin to the transform endpoint (just an echo endpoint at the moment):

```
2022-11-22T21:04:22.374Z INFO http/server.go:2084 {"apiVersion":"networking.k8s.io/v1","kind":"Ingress","metadata":{"name":"ingress-example"},"spec":{"rules":[{"host":"foo.bar.com","http":{"paths":[{"backend":{"service":{"name":"service1","port":{"number":80}}},"path":"/bar","pathType":"Prefix"}]}},{"host":"*.foo.com","http":{"paths":[{"backend":{"service":{"name":"service2","port":{"number":80}}},"path":"/foo","pathType":"Prefix"}]}}]}}
```

Tear down the environment with `make argocd-stop`

## Development

* Make changes to plugin.go
* Rebuild the plugin binary with `make argocd-cmp-plugin`

Note that the plugin binary is built for linux x86-64.
The binary is mounted from the host into the argocd-repo-server pod, where it gets executed by the argocd-cmp-server.
Any changes to the binary on the host will automatically reflect in the running ArgoCD environment.
To test out changes, you can manually trigger a refresh and/or a sync of an Application,
which should execute the plugin.

The plugin doesn't produce any logs on its own.
It's purpose is to take an input of a directory with resources, and ultimately output some trasnformed yaml that represents the desired live state of the resources.
