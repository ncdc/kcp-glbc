package route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kcp-dev/logicalcluster/v2"
	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/dns/aws"
	"github.com/kuadrant/kcp-glbc/pkg/net"
	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"
	"github.com/kuadrant/kcp-glbc/pkg/util/slice"
	"github.com/kuadrant/kcp-glbc/pkg/util/workloadMigration"
	routeapiv1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
)

type dnsReconciler struct {
	deleteDNS        func(ctx context.Context, route *routeapiv1.Route) error
	getDNS           func(ctx context.Context, route *routeapiv1.Route) (*v1.DNSRecord, error)
	createDNS        func(ctx context.Context, dns *v1.DNSRecord) (*v1.DNSRecord, error)
	updateDNS        func(ctx context.Context, dns *v1.DNSRecord) error
	watchHost        func(ctx context.Context, key interface{}, host string) bool
	forgetHost       func(key interface{}, host string)
	listHostWatchers func(key interface{}) []net.RecordWatcher
	DNSLookup        func(ctx context.Context, host string) ([]net.HostAddress, error)
	log              logr.Logger
}

func (r *dnsReconciler) reconcile(ctx context.Context, route *routeapiv1.Route) (reconcileStatus, error) {
	r.log.V(3).Info("dns reconciler start")
	if route.DeletionTimestamp != nil && !route.DeletionTimestamp.IsZero() {
		// delete DNSRecord
		if err := r.deleteDNS(ctx, route); err != nil && !k8errors.IsNotFound(err) {
			return reconcileStatusStop, err
		}
		return reconcileStatusContinue, nil
	}

	routeStatus := &routeapiv1.RouteStatus{}
	var activeHosts []string
	for k, v := range route.Annotations {
		key := routeKey(route)
		if !strings.Contains(k, workloadMigration.WorkloadStatusAnnotation) {
			continue
		}
		annotationParts := strings.Split(k, "/")
		if len(annotationParts) < 2 {
			r.log.Error(errors.New("invalid workloadStatus annotation format"), "could not process target")
			continue
		}
		//skip IP record if cluster is being deleted by KCP
		if metadata.HasAnnotation(route, workloadMigration.WorkloadDeletingAnnotation+annotationParts[1]) {
			continue
		}
		err := json.Unmarshal([]byte(v), routeStatus)
		if err != nil {
			return reconcileStatusStop, err
		}
		// Start watching for address changes in the LBs hostnames
		//mnairn - modified for routes
		//for _, ingress := range routeStatus.Ingress {
		//	if ingress.RouterCanonicalHostname != "" {
		//		r.watchHost(ctx, key, ingress.RouterCanonicalHostname)
		//		activeHosts = append(activeHosts, ingress.RouterCanonicalHostname)
		//	}
		//}
		//
		hostRecordWatchers := r.listHostWatchers(key)
		for _, watcher := range hostRecordWatchers {
			if !slice.ContainsString(activeHosts, watcher.Host) {
				r.forgetHost(key, watcher.Host)
			}
		}
	}

	// Attempt to retrieve the existing DNSRecord for this Route
	existing, err := r.getDNS(ctx, route)
	// If it doesn't exist, create it
	if err != nil {
		if !k8errors.IsNotFound(err) {
			return reconcileStatusStop, err
		}
		r.log.V(3).Info("DNSRecord does not exist")
		// doesn't exist so Create the DNSRecord object
		record := &v1.DNSRecord{}

		record.TypeMeta = metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "DNSRecord",
		}
		record.ObjectMeta = metav1.ObjectMeta{
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: logicalcluster.From(route).String(),
			},
			Name:      route.Name,
			Namespace: route.Namespace,
		}

		// Sets the Route as the owner reference
		record.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         routeapiv1.SchemeGroupVersion.String(),
				Kind:               "Route",
				Name:               route.Name,
				UID:                route.UID,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			},
		})
		if err := r.setDnsRecordFromRoute(ctx, route, record); err != nil {
			return reconcileStatusStop, err
		}
		// Create the resource in the cluster
		existing, err = r.createDNS(ctx, record)
		if err != nil {
			r.log.V(3).Info("DNSRecord create failed", "err", err)
			//ToDo mnairn, feels like we should return the err here, but we can't since it prevents the save from happening
			return reconcileStatusContinue, nil
		}
		r.log.V(3).Info("DNSRecord created!")

		// metric to observe the route admission time
		//ingressObjectTimeToAdmission.
		//	Observe(existing.CreationTimestamp.Time.Sub(route.CreationTimestamp.Time).Seconds())
		return reconcileStatusContinue, nil

	}
	// If it does exist, update it
	copyDNS := existing.DeepCopy()
	if err := r.setDnsRecordFromRoute(ctx, route, existing); err != nil {
		return reconcileStatusStop, err
	}

	if !equality.Semantic.DeepEqual(copyDNS, existing) {
		if err = r.updateDNS(ctx, existing); err != nil {
			return reconcileStatusStop, err
		}
	}

	return reconcileStatusContinue, nil
}

func (r *dnsReconciler) setDnsRecordFromRoute(ctx context.Context, route *routeapiv1.Route, dnsRecord *v1.DNSRecord) error {
	key, err := cache.MetaNamespaceKeyFunc(route)
	if err != nil {
		return fmt.Errorf("failed to get namespace key for route %s", err)
	}

	if _, ok := dnsRecord.Annotations[annotationIngressKey]; !ok {
		if dnsRecord.Annotations == nil {
			dnsRecord.Annotations = map[string]string{}
		}
		dnsRecord.Annotations[annotationIngressKey] = key
	}
	metadata.CopyAnnotationsPredicate(route, dnsRecord, metadata.KeyPredicate(func(key string) bool {
		return strings.HasPrefix(key, ANNOTATION_HEALTH_CHECK_PREFIX)
	}))
	return r.setEndpointsFromRoute(ctx, route, dnsRecord)
}

func (r *dnsReconciler) setEndpointsFromRoute(ctx context.Context, route *routeapiv1.Route, dnsRecord *v1.DNSRecord) error {
	targets, err := r.targetsFromRoute(ctx, route)
	if err != nil {
		return err
	}

	hostname := route.Annotations[ANNOTATION_HCG_HOST]

	// Build a map[Address]Endpoint with the current endpoints to assist
	// finding endpoints that match the targets
	currentEndpoints := make(map[string]*v1.Endpoint, len(dnsRecord.Spec.Endpoints))
	for _, endpoint := range dnsRecord.Spec.Endpoints {
		address, ok := endpoint.GetAddress()
		if !ok {
			continue
		}

		currentEndpoints[address] = endpoint
	}

	var newEndpoints []*v1.Endpoint

	for _, routeTargets := range targets {
		for _, target := range routeTargets {
			var endpoint *v1.Endpoint
			ok := false

			// If the endpoint for this target does not exist, add a new one
			if endpoint, ok = currentEndpoints[target]; !ok {
				endpoint = &v1.Endpoint{
					SetIdentifier: target,
				}
			}

			newEndpoints = append(newEndpoints, endpoint)

			// Update the endpoint fields
			endpoint.DNSName = hostname
			endpoint.RecordType = "A"
			endpoint.Targets = []string{target}
			endpoint.RecordTTL = 60
			endpoint.SetProviderSpecific(aws.ProviderSpecificWeight, awsEndpointWeight(len(routeTargets)))
		}
	}

	dnsRecord.Spec.Endpoints = newEndpoints
	return nil
}

// targetsFromRoute returns a map of all the IPs associated with a single route(cluster)
func (r *dnsReconciler) targetsFromRoute(ctx context.Context, route *routeapiv1.Route) (map[string][]string, error) {
	targets := map[string][]string{}
	deletingTargets := map[string][]string{}

	routeStatus := &routeapiv1.RouteStatus{}
	//find all annotations of a workload status (indicates a synctarget for this resource)
	_, annotations := metadata.HasAnnotationsContaining(route, workloadMigration.WorkloadStatusAnnotation)
	for k, v := range annotations {
		//get the cluster name
		annotationParts := strings.Split(k, "/")
		if len(annotationParts) < 2 {
			r.log.Error(errors.New("invalid workloadStatus annotation format"), "skipping sync target")
			continue
		}
		clusterName := annotationParts[1]

		err := json.Unmarshal([]byte(v), routeStatus)
		if err != nil {
			return nil, err
		}
		statusTargets, err := r.targetsFromRouteStatus(ctx, routeStatus)
		if err != nil {
			return nil, err
		}

		if metadata.HasAnnotation(route, workloadMigration.WorkloadDeletingAnnotation+clusterName) {
			for host, ips := range statusTargets {
				deletingTargets[host] = append(deletingTargets[host], ips...)
			}
			continue
		}
		for host, ips := range statusTargets {
			targets[host] = append(targets[host], ips...)
		}
	}
	//no non-deleting hosts have an IP yet, so continue using IPs of "losing" clusters
	if len(targets) == 0 && len(deletingTargets) > 0 {
		return deletingTargets, nil
	}

	return targets, nil
}

func (r *dnsReconciler) targetsFromRouteStatus(ctx context.Context, routeStatus *routeapiv1.RouteStatus) (map[string][]string, error) {
	targets := map[string][]string{}
	//mnairn - modified for routes
	for _, ingress := range routeStatus.Ingress {
		if ingress.RouterCanonicalHostname != "" {
			ips, err := r.DNSLookup(ctx, ingress.RouterCanonicalHostname)
			if err != nil {
				return nil, err
			}
			targets[ingress.RouterCanonicalHostname] = []string{}
			for _, ip := range ips {
				targets[ingress.RouterCanonicalHostname] = append(targets[ingress.RouterCanonicalHostname], ip.IP.String())
			}
		}
	}
	return targets, nil
}

// awsEndpointWeight returns the weight value for a single AWS record in a set of records where the traffic is split
// evenly between a number of clusters/ingresses, each splitting traffic evenly to a number of IPs (numIPs)
//
// Divides the number of IPs by a known weight allowance for a cluster/ingress, note that this means:
// * Will always return 1 after a certain number of ips is reached, 60 in the current case (maxWeight / 2)
// * Will return values that don't add up to the total maxWeight when the number of ingresses is not divisible by numIPs
//
// The aws weight value must be an integer between 0 and 255.
// https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-values-weighted.html#rrsets-values-weighted-weight
func awsEndpointWeight(numIPs int) string {
	maxWeight := 120
	if numIPs > maxWeight {
		numIPs = maxWeight
	}
	return strconv.Itoa(maxWeight / numIPs)
}

func (c *Controller) updateDNS(ctx context.Context, dns *v1.DNSRecord) error {
	if _, err := c.dnsRecordClient.Cluster(logicalcluster.From(dns)).KuadrantV1().DNSRecords(dns.Namespace).Update(ctx, dns, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

func (c *Controller) deleteDNS(ctx context.Context, route *routeapiv1.Route) error {
	return c.dnsRecordClient.Cluster(logicalcluster.From(route)).KuadrantV1().DNSRecords(route.Namespace).Delete(ctx, route.Name, metav1.DeleteOptions{})
}

func (c *Controller) getDNS(ctx context.Context, route *routeapiv1.Route) (*v1.DNSRecord, error) {
	return c.dnsRecordClient.Cluster(logicalcluster.From(route)).KuadrantV1().DNSRecords(route.Namespace).Get(ctx, route.Name, metav1.GetOptions{})
}

func (c *Controller) createDNS(ctx context.Context, dnsRecord *v1.DNSRecord) (*v1.DNSRecord, error) {
	return c.dnsRecordClient.Cluster(logicalcluster.From(dnsRecord)).KuadrantV1().DNSRecords(dnsRecord.Namespace).Create(ctx, dnsRecord, metav1.CreateOptions{})
}
