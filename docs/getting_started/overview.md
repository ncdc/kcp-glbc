# What is KCP?

Kube-like Control Plane (KCP) is a prototype of a multi-tentant Kubernetes control plane for workloads on many clusters.

It provides a generic CustomResourceDefinition (CRD) apiserver that is divided into multiple logical clusters that enable multitenancy of cluster-scoped resources such as CRDs and Namespaces.

Each of these logical clusters is fully isolated from the others, allowing different teams, workloads, and use cases to live side by side.

To learn more about the terminology, refer to the [docs](https://github.com/kcp-dev/kcp/blob/main/docs/terminology.md).

KCP can be used to manage Kubernetes-like applications across one or more clusters and integrate with cloud services. To an end user, kcp should appear to be a normal cluster (supports the same APIs, client tools, and extensibility) but allows you to move your workloads between clusters or span multiple clusters without effort. kcp lets you keep your existing workflow and abstract Kube clusters like a Kube cluster abstracts individual machines. kcp also helps broaden the definition of "Kubernetes applications" by being extensible, only loosely dependent on nodes, pods, or clusters, and thinking more broadly about what an application is than "just some containers".

# What is GLBC?

The KCP Global Load Balancer Controller (GLBC) solves multi cluster ingress use cases when leveraging KCP to provide transparent multi cluster deployments.

The main use case it solves currently is providing you with a single host that can be used to access your workload and bring traffic to the correct physical clusters. The GLBC manages the DNS for this host and provides you with a valid TLS certificate. If your workload moves/is moved or expands contracts across clusters, GLBC will ensure that the DNS for this host is correct and traffic will continue to reach your workload.

Currently, the GLBC is deployed in a Kubernetes cluster, referred as the GLBC control cluster, outside the KCP control plane. The GLBC dependencies, such as cert-manager, and eventually external-dns, are deployed alongside the GLBC in that control cluster.

These components coordinate via a shared state, that's persisted in the control cluster data plane.

The following benefits are envisioned:

Leverage the data durability guarantees, provided by hosted KCP environments;
Compute commoditization, and workload movement.

# Architecture

<Insert Diagram>


# Terms to Know
