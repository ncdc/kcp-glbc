## Troubleshooting

### "make local-setup" command.
While attempting to run make local-setup it’s possible you will encounter some of the following errors:
<br><br>

**make: *** No rule to make target 'local-setup':**
After cloning the repo, make sure to run the “make local-setup” command in the directory where the repo was cloned.<br><br>


**bash: line 1: go: command not found:**
We must install the correct go version used for this project. The version number can be found in the go.mod file of the repo. In this case, it is go 1.17.
If running on Fedora, here is a [guide to install go on Fedora 36](https://nextgentips.com/2022/05/21/how-to-install-go-1-18-on-fedora-36/). Before running the command to install go, make sure to type in the correct go version that is needed.<br><br>


**kubectl: command not found:**
Here is a quick and easy way of [installing kubectl on Fedora](https://snapcraft.io/install/kubectl/fedora).<br><br>


**Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?:**
Run the following command to start Docker daemon:
```bash
sudo systemctl start docker
```
<br><br>
**Kind cluster failed to become ready - Check logs for errors:**
Attempt the following to confirm if *kcp-cluster-1* and *kcp-cluster-2* are in a READY state:
```bash
KUBECONFIG=.kcp/admin.kubeconfig ./bin/kubectl-kcp workspace use root:kuadrant
Current workspace is "root:kuadrant".
```
```bash
kubectl get synctargets -o wide
NAME            LOCATION        READY   SYNCED API RESOURCES
kcp-cluster-1   kcp-cluster-1   True    
kcp-cluster-2   kcp-cluster-2   False 
```
If a cluster is not in READY state, the following procedure might solve the issue: [Configure Linux for Many Watch Folders](https://www.ibm.com/docs/en/ahte/4.0?topic=wf-configuring-linux-many-watch-folders) (we want to bump up each of the limits).
