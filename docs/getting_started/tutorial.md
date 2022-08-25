# What is GLBC?

The Global Load Balancer Controller (GLBC) leverages [KCP](https://github.com/Kuadrant/kcp) to provide DNS-based global load balancing and transparent multi-cluster ingress. The main API for the GLBC is the Kubernetes Ingress object. GLBC identifies ingress objects and transforms them adding in the GLBC managed host and TLS certificate.

For more information on the architecture of GLBC and how the various component work, refer to the [overview documentation](https://github.com/Kuadrant/kcp-glbc/blob/bb8e43639691568b594720244a0c94a23470a587/docs/getting_started/overview.md).

Use this tutorial to perform the following actions:

* Install the KCP-GLBC instance and verify installation.
* Follow the demo and have GLBC running and working with an AWS domain. You can then deploy the sample service to view how GLBC allows access to services  in a multi-cluster ingress scenario.

---

# Install GLBC

## Prerequisites

- Ensure you clone [this](https://github.com/Kuadrant/kcp-glbc.git) repository KCP-GLBC with the release tag `KCP 0.5.0`
- Install [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).
- Install Go `1.17` or higher. This is the version used in KCP-GLBC as indicated in the [`go.mod`](https://github.com/Kuadrant/kcp-glbc/blob/main/go.mod) file.
- You have an AWS account, a DNS Zone, and a subdomain for the domain being used. You will need this in order to instruct GLBC to validate your AWS credentials and configuration.


## Installation

1. Clone the [repository](https://github.com/Kuadrant/kcp-glbc.git).
1. Run the following command:

   ```bash
   make local-setup
   ```

> NOTE: If errors are encountered while running the locall-setup command, refer to the Troubleshooting Installation document.

The script performs the following actions:

* Build all the binaries.
* Deploy three Kubernetes `1.22` clusters locally using [kind](https://kind.sigs.k8s.io/docs/user/quick-start/).
* Deploy and configure the ingress controllers in each cluster.
* Start the KCP server.
* Create KCP workspaces for GLBC and user resources:
    * `kcp-glbc`
    * `kcp-glbc-compute`
    * `kcp-glbc-user`
    * `kcp-glbc-user-compute`
* Add workload clusters to the `*-compute` workspaces
    * kcp-glbc-compute: 1x kind cluster
    * kcp-glbc-user-compute: 2x kind clusters
* Deploy GLBC dependencies (`cert-manager`) into the KCP-GLBC workspace.


## Verify Installation

1. Verify that the ingress was created after deploying the sample service:
   ```bash
   kubectl get ingress
   ```
1. You can also view in your AWS domain if the DNS record was created.

   ![Screenshot from 2022-07-28 19-40-03](https://user-images.githubusercontent.com/73656840/181613546-4257b69c-a824-4d76-bf08-d56f70771ef3.png)

---

## Deploy GLBC

After the `local-setup` has successfully completed, you will receive a message that KCP is now running. At this point, GLBC is not yet running. You will be presented with two options to deploy GLBC:

1. [Local-deployment](https://github.com/Kuadrant/kcp-glbc/blob/main/docs/local_deployment.md): This option is good for testing purposes by using a local KCP instance and kind clusters.

1. [Deploy latest in KCP](https://github.com/Kuadrant/kcp-glbc/blob/main/docs/deployment.md) with monitoring enabled: This will deploy GLBC to your target KCP instance, and allow you to view observability in Prometheus and Grafana.


### Add AWS credentials

For the demo, before deploying GLBC, you will need to provide your AWS credentials and configuration. The easiest way to do this is to perform the folowing steps:

1. Open the KCP-GLBC project in your IDE. 
1. Navigate to the `./config/deploy/local/aws-credentials.env` environment file.
1. Enter your `AWS Access Key ID` and `AWS Secret Access Key` as indicated in the example below:
   ![Screenshot from 2022-07-28 12-33-50](https://user-images.githubusercontent.com/73656840/181609265-8577f9c0-1d32-4e1f-8cf2-7542a340393b.png)

1. Navigate to `./config/deploy/local/controller-config.env` and change the fields to something similar to this:

   ![Screenshot from 2022-07-28 12-43-56](https://user-images.githubusercontent.com/73656840/181609374-b0d2c81f-0d46-4816-b53e-05514fa382c2.png)

   The fields that might need to be edited include:
   - Replace `<AWS_DNS_PUBLIC_ZONE_ID>` with your own hosted zone ID.
   - Replace `<GLBC_DNS_PROVIDER>`
   - Replace `<GLBC_DOMAIN>` with your specified subdomain

       ![Screenshot from 2022-07-28 12-43-16](https://user-images.githubusercontent.com/73656840/181609406-7fc7f32b-001e-4da8-b423-fdb00b11228f.png)

 1. After the above is correctly configured, copy the commands from the output in the terminal under _Option 2_, and run it in a new tab use `controller-config.env` and `aws-credentials.env` in GLBC. This way, you will be able to curl the domain in the tutorial and visualize how the workload from cluster-1 migrates to cluster-2. The commands are similar to the following:

   ![Screenshot of cluster migration](https://user-images.githubusercontent.com/73656840/181609752-1b4d481a-41bf-4de6-aba6-a8e0d004724e.png)

 1. After running `local-setup` successfully and while running GLBC running, deploy the sample service. You can do this by copying each command under _sample service_ and running them in a new tab in the terminal. The commands will look similar to the following:

   ![Screenshot from 2022-07-28 14-42-57](https://user-images.githubusercontent.com/73656840/181609847-518076be-c1de-4894-b44e-2fcd4a2f80e8.png)


## Demo on providing ingress in a multi-cluster ingress scenario

This section will show how GLBC is used to provide ingress in a multi-cluster ingress scenario.

> NOTE: This version of the tutorial works with `KCP 0.5.0`

For this tutorial, after following the [Installation](#install-GLBC) section of this document, we should already have KCP and GLBC running, and also have had deployed the sample service which would have created an ingress named *"ingress-nondomain"*. 

> NOTE: The `default` namespace is where you are putting all the sample resources at the moment.


### Viewing the `default` namespace

We will run the following command `kubectl edit ns default -o yaml` in the same tab where we deployed the sample service to view and edit the "*default*" namespace.

As we can see, there is a label named: "*state.internal.workload.kcp.dev/kcp-cluster-1: Sync*":

![Screenshot from 2022-07-28 19-51-26](https://user-images.githubusercontent.com/73656840/181615489-f8472982-cbfd-4920-98f6-2aba53df79a4.png)

GLBC is telling KCP where to sync all of the work resources in the namespace to. Meaning, since the namespace has *kcp-cluster-1* set on it, the ingress will also have *kcp-cluster-1* set on it. 


### Curl the running domain

In a new tab in the terminal, we can curl the domain to view it running. To do this, we will copy the address from our new DNS record in our AWS:

![Screenshot from 2022-07-28 19-43-22](https://user-images.githubusercontent.com/73656840/181614120-5a8df2fc-02e7-4fa5-8f39-4d965890f7ba.png)


Then, we will run the command similar to the example below to continue to curl the domain every 10 seconds:

`watch -n 10 curl -k https://cbhboulkjgm0gb16jm3g.cz.hcpapps.net`

Which gives an output similar to this one:

![Screenshot from 2022-07-28 19-42-41](https://user-images.githubusercontent.com/73656840/181614019-46734d7b-8557-401e-9e02-fd68683aba23.png)

This would mean that kcp-cluster-1 is up and running correctly.

<br>

### Editing the "default" namespace and viewing the ingress

We can now go back to the tab where we are viewing the "default" namespace and edit the label mentioned before: "*state.internal.workload.kcp.dev/kcp-cluster-1: Sync*", and change it from *kcp-cluster-1* to *kcp-cluster-2*, and exit and save.

![Screenshot from 2022-07-26 11-46-27](https://user-images.githubusercontent.com/73656840/180992544-c21516fa-85a0-4c6a-9abc-efeb1a7c3433.png)

We will notice that in the tab where we are curling the domain, it now outputs a 404 Not Found error as the workload gets removed from *kcp-cluster-1* and migrates to *kcp-cluster-2*. The DNS record will update and in a few seconds we will start getting a response from *kcp-cluster-2*.

![Screenshot from 2022-07-28 19-38-14](https://user-images.githubusercontent.com/73656840/181613118-c692cf38-cea4-455b-b2f3-51886f15cca5.png)


In the meantime, in the tab where we ran the sample service, we can run the following command to view the ingress "*ingress-nondomain*": `kubectl get ingress ingress-nondomain -o yaml`. 

We can observe that the label in the ingress has changed from *kcp-cluster-1* to *kcp-cluster-2*. KCP has propagated that label down from the namespace to everything in it. Everything in the namespace gets the same label. Because of that label, KCP is syncing it to *kcp-cluster-2*.

![Screenshot from 2022-07-26 11-47-30](https://user-images.githubusercontent.com/73656840/180992725-c6a4f985-da9f-4b68-bda7-ed3e61f43499.png)


Moreover, In the annotations we also have a status there for *kcp-cluster-2* and it has an IP address in it meaning that it has indeed synced to *kcp-cluster-2*. We will also find another annotation named something like "*kuadrant.dev/glbc-delete-at-kcp-cluster-1: 1658757564*", which is code coming from the GLBC which is saying "Don't remove this work from *kcp-cluster-1* until the DNS has propagated."

For that reason we can also observe another annotation named "*finalizers.workload.kcp.dev/kcp-cluster-1: kuadrant.dev/glbc-migration*" which remains there because GLBC is saying to KCP "Don't get rid of this yet, as we're waiting for it to come up to another cluster before you remove it from *kcp-cluster-1*" Once it has completely migrated, the finalizer for *kcp-cluster-1* will no longer be there.

![Screenshot from 2022-07-26 11-48-57](https://user-images.githubusercontent.com/73656840/180993006-78f47abc-d006-4045-95b7-33428cf65d6b.png)


By now, we should have a response back from *kcp-cluster-2*, which would mean that the workload has successfully migrated from cluster-1 to cluster-2.

   ![Screenshot from 2022-07-28 19-56-10](https://user-images.githubusercontent.com/73656840/181616186-0921ad19-53d9-4b6b-8fee-012517e2878c.png)

