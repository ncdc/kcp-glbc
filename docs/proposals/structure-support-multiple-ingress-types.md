# Global Ingress Controller Code Restructure

**Authors:** Phil Brookes <pbrookes@redhat.com>, Craig Brookes <cbrookes@redhat.com>


**Terms**
- **Traffic Routing Object:** Covers OpenShift Routes, k8s Ingresses and Gateway API HTTPRoutes
- **Mutator:** Will perform business logic based on Traffic Routing Objects

## Problem


There are multiple traffic routing objects that can be used to configure how traffic is routed in a cluster (OpenShift Route, HTTPRoute and Ingress). We need to be able to  provide the same functionality to each  of these APIs with as little code duplication as possible.

Our code and pkg structure has evolved without too much consideration to the structure, this has been helpful while iterating and understanding the problem domains we are working within. We are at a point where we understand the domains and so our pkg structure can evolve to clearly represent those and avoid ambiguous packages such as util | common | misc, multiple packages with the same names and pkg names that represent code structure (e.g reconciler).


## Proposed Solution

- Have a clear set of packages that have responsibility for the individual domain.
    - **Traffic:**
        - converts traffic routing objects to ReadWriter interface implementations and passes them off to `Mutators`.
        - Root event handler for all traffic routing objects (update, delete, add)
    - **TLS:**
        - Owns tls certificates and applying any mutations to the traffic routing object for TLS. 
    - **DNS:**
        - Owns DNSRecord reconciliation and any mutations to the traffic routing object for DNS
    - **Migration:**
        - Handles advanced scheduling mutations to the traffic routing object
    - **Domains:**
        - Handles custom domain logic and owns domain verification reconciliation.
        - Responsible for any domain / host mutations such as assigning the managed sub domain to the traffic routing object.

- Define a ReadWriter generic interface that hides the details of each specific traffic routing object behind a single interface.
- Define a Mutator interface that anything within a domain wanting to mutate a routing object must implement.
- Have a single top level controller that handles events for all traffic routing objects that has a simple reconcile method that converts from the specifc routing object into the generic ReadWriter interface and hands off to the mutators. It will also handle events for re-queuing these objects (similar to what the ingress controller does now).
- Have an `internal` pkg that is for plumbing items such as logging


**Mutator**:

```
type Mutator interface{
    Mutate(apis.ReadWriter)error
}

```


```
cmd/ # no change (binaries and main.go)

internal/ # stuff that is not to do with our core "business domains"
    - logger/log.go
    - metrics/server.go
    - admission/server.go
    
pkg/apis/kuadrant/v1 
    - models
    ...
pkg/apis/
    - ReadWriter #interface used by mutators and the traffic controller to read and write to traffic routing objects
pkg/dns/ #dnsRecord reconciler 
    - mutator.go #implements mutator
    - controller #dnsRecord Controller
    - reconciler #dnsRecordReconciler
    ...
    
pkg/tls/
    - mutator.go #implements mutator 
    ...

pkg/domains/ #handles setting the hosts and custom domains
    - mutator.go #implements mutator takes a readWriter
    - controller #domain verification controller
    - reconciler #domain verification reconciler
    ...
pkg/migration/ # for migraton policy for example
    - mutator.go #implements mutator changes routing objects
    - controller.go deployments, secrets, services, etc. (reconciles migration)
    
pkg/traffic/
    - controller.go #converts traffic resource to a readWriter implementation which calls out to all mutators
    - mutator #interface - business logic that mutates the resource via the readWriter interface
    - ingressReadWriter #implementation of all readWriter interfaces for an ingress
    - routeReadWriter #implementation of all readWriter interfaces for a route
    - httprouteReadWriter #implementation of all readWriter interfaces for a httproute
```

### controller view
User creates ingress object
Traffic Controller event handler is fired
Traffic Controller Converts traffic resource to IngressReadWriter that is an implementation of the ReadWriter interface
Traffic Controller Passes IngressReadWriter to Reconciler
Reconciler composes mutators 
Reconciler passes readwriter to mutators
Reconciler controls flow and whether it should continue
Ingress Controller updates Ingress against API
**Note** this is not significantly different than what we have now but it formalises the pattern

## example mutator scenarios 
### DNS mutator view
Receives readwriter object
Reconciles DNS Record based on object properties
Updates status via readwriter
Returns error or nil

### Domains mutator view
Receives readwriter object
ensure a managed host annotation exists on readwriter
Ensures all hosts in readwriter spec are verified
moves unverfied hosts to pending hosts annotation via readWriter
ensures each rule has a managed host clone
Returns error or nil


### TLS mutator view
Receives readwriter object
Creates a certificate resource based on readWriter
Updates the status of the readwriter
updates the tls section of the readWriter
Returns error or nil
