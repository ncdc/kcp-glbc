# Ingress Resources and Behavior

This document covers the ``` networking.k8s.io/v1 Ingress``` resource and the behavior of global load balancing controller (GLBC) when handling the Ingress resource via [KCP](https://github.com/kcp-dev/kcp). This behavior is applied to all Ingress objects created within a KCP workspace when the GLBC is enabled in that workspace.

**Definitions:**
    
### Managed Domain:

The managed domain is the root domain that the GLBC controls the DNS Zone for. 

### Managed Host:
GLBC hands out subdomains of the managed domain that are used as managed hosts. The DNS for this managed host is also controlled by GLBC. It is this managed host that you will always be able to use to access your workload across clouds and clusters.

A managed domain may be something like ```hcpapps.net``` and managed host would look something like ```<guid>.hcpapps.net```

### Custom Domain

A custom domain, is a domain controlled by the end user. GLBC does not control the DNS for these domains. Custom domains can be used in combination with a CNAME to the managed host. 
An example custom domain would be something like ```myapp.com``` or ```apps.myapps.net```

### Behavior

In order to provide multi cluster ingress, the GLBC interacts with certain fields within the K8s Ingress object and also expects certain rules to be followed when defining an Ingress object. Below is outlined how GLBC works with certain fields of the Ingress object, what you can expect to see with an Ingress object managed by GLBC and any known limitations that are present.

## Rule Blocks

While there are some annotations created by GLBC, the main interaction points are with the Ingress rules blocks

### The Host Field

This is the main field that the GLBC interacts with. By default for each defined rules block within an Ingress, the GLBC will set the host field to be a managed host.

```
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: Ingress-domain
spec:
  rules:
    - host: <guid>.hcpapps.net #example
    ...
```    


### No hosts specified
You can define an Ingress with a rule block that has an empty host field. When GLBC sees a rule block with an empty host field, a managed host will be generated and injected into the rule block. 

There should be only one rule block with an empty value. If there are multiple rules blocks with empty values, by default the same managed host will be injected into each rule block within a given Ingress that has no host specified. It is the responsibility of the end user to ensure any path rules within those blocks do not conflict. 

If you have a need for multiple rules blocks with no specified host, it is suggested you create multiple Ingress objects. Each Ingress object will receive its own unique managed host.

### Specifying a host
For each rules block within an Ingress definition, If you have specified a value for the host field, by default GLBC will replace that value with a managed host unless a DNS based domain verification has been completed. 

Once a custom domain has been verified (see custom domain documentation for more on this process), GLBC will re-add the rules block that was replaced alongside the rules block with the managed host. To direct traffic from your custom domain to your application, you need to setup a CNAME record for your custom domain. This CNAME record can be any of the managed hosts within the namespace. This is because KCP will schedule all workloads within a namespace to the same workload clusters. 

For more info and to better understand using custom domains see the custom domain documentation (link todo) 


### Multiple Ingresses

Multiple GLBC managed Ingress objects within a given namespaces is supported. Each individual Ingress will receive its own unique managed host. 
When using a custom domain, multiple Ingresses with the same custom domain is also supported with the following limitations. 
- In order for your custom domain to be used to send traffic to your application(s) you will need to setup a CNAME record. The target for that CNAME should be one of the managed hosts within the namespace where the custom domain is being used. Any managed host can be selected as by default KCP schedules everything within a single namespace to the same workload clusters.
- Using the same custom domain across multiple namespaces is not recommended at this point as (depending on what synctargets and locations you have setup) the workloads could be scheduled to different workload clusters. This means the DNS may not be correct to send traffic for both applications deployed across different namespaces.
- If you delete the ingress that has the managed host you selected as the CNAME, you will need to update the CNAME record to a different managed host within the namespace for your application to remain reachable on the custom domain. 

Below is an example to illustrate (note there are two different Ingress objects within a single namespace)

```
#Ingress1 using a custom domain.

apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: Ingress1
  namespace: test
spec:
  rules:
    - host: app.myapp.com #example custom domain
    ...    
    - host: 1234.hcpapps.net #example managed host this will resolve to the loadbalancers in front of the synctargets where the workloads and ingresses are scheduled
    ...

---

#Ingress2 created with verfied custom domain in its rule block

apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: Ingress2
  namespace: test
spec:
  rules:
    - host: app.myapp.com #custom domain reused in the same namespace
    ...    
    - host: 1235.hcpapps.net #different managed host but will also resolve to the loadbalancers infront of the workload clusters selected for this namespace.
    ...
```


## Backends

You can define multiple backends just as you would for a regular Ingress with the following caveats. The limitation here is that in the context of KCP, each of these targeted backends within a single Ingress object have to be placed on the same cluster for an Ingress with multiple backends to work as intended. This is the default with KCP scheduling currently. Scheduling happens at the namespace level. So there should be no issues. 



## TLS Support

By default GLBC will generate a valid certificate for the managed host and inject this certificate via a secret into the Ingress object.
If you have added a custom tls section for a custom domain, this will be removed initially pending a domain verification. Once your custom domain is verified, the tls section will be restored along side the managed domain rules block. GLBC wont do anything specific with the secret you created to contain the certificate, it will only work with the definition of the Ingress Spec.
