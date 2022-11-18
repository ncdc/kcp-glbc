# ArgoCD plugin dev environment

## Setup

Start a kind cluster and install argocd in it. ArgoCD UI is accessible at https://localhost:8080

`make argocd-start`

Show the logs of the plugin. It might take a moment for the pod to start.

`make argocd-cmp-logs`

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
time="2022-11-18T17:30:55Z" level=info msg="argocd-glbc-plugin generate ." dir=/tmp/_cmp_server/09a77db4-6dc5-4cb6-8b62-d2948282d47b/manifests/test-app execID=00dfa
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
