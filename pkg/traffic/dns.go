package traffic

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	"github.com/kcp-dev/logicalcluster/v2"

	workload "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	"github.com/kuadrant/kcp-glbc/pkg/_internal/metadata"
	"github.com/kuadrant/kcp-glbc/pkg/_internal/slice"
	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/dns"
	"github.com/kuadrant/kcp-glbc/pkg/dns/aws"
	"github.com/kuadrant/kcp-glbc/pkg/traffic/geo/ipwhois"
)

type DnsReconciler struct {
	DeleteDNS        func(ctx context.Context, accessor Interface) error
	GetDNS           func(ctx context.Context, accessor Interface) (*v1.DNSRecord, error)
	CreateDNS        func(ctx context.Context, dns *v1.DNSRecord) (*v1.DNSRecord, error)
	UpdateDNS        func(ctx context.Context, dns *v1.DNSRecord) (*v1.DNSRecord, error)
	WatchHost        func(ctx context.Context, key interface{}, host string) bool
	ForgetHost       func(key interface{}, host string)
	ListHostWatchers func(key interface{}) []dns.RecordWatcher
	DNSLookup        func(ctx context.Context, host string) ([]dns.HostAddress, error)
	Log              logr.Logger
}

func (r *DnsReconciler) GetName() string {
	return "DNS Reconciler"
}

func (r *DnsReconciler) Reconcile(ctx context.Context, accessor Interface) (ReconcileStatus, error) {
	if accessor.GetDeletionTimestamp() != nil && !accessor.GetDeletionTimestamp().IsZero() {
		if err := r.DeleteDNS(ctx, accessor); err != nil && !k8errors.IsNotFound(err) {
			return ReconcileStatusStop, err
		}
		return ReconcileStatusContinue, nil
	}

	key := objectKey(accessor)
	managedHost := accessor.GetAnnotations()[ANNOTATION_HCG_HOST]
	var activeLBHosts []string
	activeDNSTargetIPs := map[string][]string{}
	deletingTargetIPs := map[string][]string{}
	activeDNSTargets := map[string]dns.Target{}

	targets, err := accessor.GetTargets(ctx, r.DNSLookup)
	if err != nil {
		return ReconcileStatusContinue, err
	}
	for cluster, targets := range targets {
		deleteAnnotation := workload.InternalClusterDeletionTimestampAnnotationPrefix + cluster.String()
		if metadata.HasAnnotation(accessor, deleteAnnotation) {
			for host, target := range targets {
				deletingTargetIPs[host] = append(deletingTargetIPs[host], target.Value...)
			}
			continue
		}
		for host, target := range targets {
			if metadata.HasAnnotation(accessor, deleteAnnotation) {
				continue
			}
			if target.TargetType == dns.TargetTypeHost {
				r.WatchHost(ctx, key, host)
				activeLBHosts = append(activeLBHosts, host)
			}
			activeDNSTargetIPs[host] = append(activeDNSTargetIPs[host], target.Value...)
			activeDNSTargets[host] = target
		}
	}

	// no non-deleting hosts have an IP yet, so continue using IPs of "losing" clusters
	if len(activeDNSTargetIPs) == 0 && len(deletingTargetIPs) > 0 {
		r.Log.V(3).Info("setting the dns Target to the deleting Target as no new dns targets set yet")
		activeDNSTargetIPs = deletingTargetIPs
	}
	// clean up any watchers no longer needed
	hostRecordWatchers := r.ListHostWatchers(key)
	for _, watcher := range hostRecordWatchers {
		if !slice.ContainsString(activeLBHosts, watcher.Host) {
			r.ForgetHost(key, watcher.Host)
		}
	}
	// Attempt to retrieve the existing DNSRecord for this traffic object
	existing, err := r.GetDNS(ctx, accessor)
	// If it doesn't exist, create it
	if err != nil {
		if !k8errors.IsNotFound(err) {
			return ReconcileStatusStop, err
		}
		record, err := newDNSRecordForObject(accessor)
		if err != nil {
			return ReconcileStatusContinue, err
		}
		r.setEndpointFromTargets(managedHost, activeDNSTargetIPs, activeDNSTargets, record)
		// Create the resource in the cluster
		if len(record.Spec.Endpoints) > 0 {
			r.Log.V(3).Info("creating DNSRecord ", "record", record.Name, "endpoints for DNSRecord", record.Spec.Endpoints)
			existing, err = r.CreateDNS(ctx, record)
			if err != nil {
				return ReconcileStatusContinue, err
			}
			// metric to observe the accessor admission time
			IngressObjectTimeToAdmission.Observe(existing.CreationTimestamp.Time.Sub(accessor.GetCreationTimestamp().Time).Seconds())
		}
		return ReconcileStatusContinue, nil
	}
	// If it does exist, update it
	copyDNS := existing.DeepCopy()
	r.setEndpointFromTargets(managedHost, activeDNSTargetIPs, activeDNSTargets, copyDNS)
	objMeta, err := meta.Accessor(accessor)
	if err != nil {
		return ReconcileStatusContinue, err
	}
	copyHealthAnnotations(copyDNS, objMeta)
	if !equality.Semantic.DeepEqual(copyDNS, existing) {
		r.Log.V(3).Info("updating DNSRecord ", "record", copyDNS.Name, "endpoints for DNSRecord", "endpoints", copyDNS.Spec.Endpoints)
		if _, err = r.UpdateDNS(ctx, copyDNS); err != nil {
			return ReconcileStatusStop, err
		}
	}

	return ReconcileStatusContinue, nil
}

func newDNSRecordForObject(obj runtime.Object) (*v1.DNSRecord, error) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	record := &v1.DNSRecord{}

	record.TypeMeta = metav1.TypeMeta{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       "DNSRecord",
	}
	objGroupVersion := schema.GroupVersion{Group: obj.GetObjectKind().GroupVersionKind().Group, Version: obj.GetObjectKind().GroupVersionKind().Version}
	// Sets the Ingress as the owner reference
	record.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         objGroupVersion.String(),
			Kind:               obj.GetObjectKind().GroupVersionKind().Kind,
			Name:               objMeta.GetName(),
			UID:                objMeta.GetUID(),
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		},
	})
	record.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			logicalcluster.AnnotationKey: logicalcluster.From(objMeta).String(),
		},
		Name:      objMeta.GetName(),
		Namespace: objMeta.GetNamespace(),
	}
	if _, ok := record.Annotations[ANNOTATION_TRAFFIC_KEY]; !ok {
		if record.Annotations == nil {
			record.Annotations = map[string]string{}
		}
		record.Annotations[ANNOTATION_TRAFFIC_KEY] = string(objectKey(obj))
	}

	copyHealthAnnotations(record, objMeta)
	return record, nil

}

func copyHealthAnnotations(dnsRecord *v1.DNSRecord, objectMeta metav1.Object) {
	metadata.CopyAnnotationsPredicate(objectMeta, dnsRecord, metadata.KeyPredicate(func(key string) bool {
		return strings.HasPrefix(key, ANNOTATION_HEALTH_CHECK_PREFIX)
	}))
}

func (r *DnsReconciler) setEndpointFromTargets(dnsName string, dnsTargets map[string][]string, activeDNSTargets map[string]dns.Target, dnsRecord *v1.DNSRecord) {
	currentEndpoints := make(map[string]*v1.Endpoint, len(dnsRecord.Spec.Endpoints))

	//dnsTargets = "host/ip -> array of ips"
	r.Log.V(3).Info("setEndpointFromTargets", "dnsName", dnsName, "dnsTargets", dnsTargets)

	//geoDnsTargets = "continent code -> host/ip -> array of ips"
	geoDnsTargets := map[string]map[string][]string{}
	for host, targets := range dnsTargets {

		r.Log.V(3).Info("setEndpointFromTargets", "activeDNSTargets[host].TargetMeta.Geo", activeDNSTargets[host].TargetMeta.Geo)

		targetContinentCode := activeDNSTargets[host].TargetMeta.Geo.ContinentCode
		if targetContinentCode == "" {
			targetContinentCode = awsGeoContinentCode(targets[0])
			r.Log.V(3).Info("setEndpointFromTargets, using ip lookup", "targetContinentCode", targetContinentCode)
		} else {
			r.Log.V(3).Info("setEndpointFromTargets, using annotation", "targetContinentCode", targetContinentCode)
		}

		if geoDnsTargets[targetContinentCode] == nil {
			geoDnsTargets[targetContinentCode] = map[string][]string{}
		}
		geoDnsTargets[targetContinentCode][host] = targets
	}

	continentCodes := make([]string, 0, len(geoDnsTargets))
	for k := range geoDnsTargets {
		continentCodes = append(continentCodes, k)
	}
	sort.Strings(continentCodes)

	r.Log.V(3).Info("setEndpointFromTargets", "geoDnsTargets", geoDnsTargets, "continentCodes", continentCodes)

	for _, endpoint := range dnsRecord.Spec.Endpoints {
		currentEndpoints[endpoint.SetID()] = endpoint
	}
	var (
		newEndpoints []*v1.Endpoint
		endpoint     *v1.Endpoint
	)
	ok := false
	for continentCode, hosts := range geoDnsTargets {
		r.Log.V(3).Info("setEndpointFromTargets", "hosts", hosts, "continentCode", continentCode)

		p := strings.Split(dnsName, ".")
		first := p[0]
		copy(p, p[1:])
		p = p[:len(p) - 1]
		dnsNameContinent := fmt.Sprintf("%s.%s.%s", first, strings.ToLower(continentCode), strings.Join(p, "."))

		r.Log.V(3).Info("setEndpointFromTargets", "dnsNameContinent", dnsNameContinent)

		if endpoint, ok = currentEndpoints[dnsNameContinent]; !ok {
			endpoint = &v1.Endpoint{
				SetIdentifier: dnsNameContinent,
			}
		}
		endpoint.DNSName = dnsName
		endpoint.RecordType = "CNAME"
		endpoint.Targets = []string{dnsNameContinent}
		endpoint.RecordTTL = 60
		endpoint.SetProviderSpecific(aws.ProviderSpecificGeolocationContinentCode, continentCode)
		newEndpoints = append(newEndpoints, endpoint)

		if continentCode == continentCodes[0] {
			if endpoint, ok = currentEndpoints["default"]; !ok {
				endpoint = &v1.Endpoint{
					SetIdentifier: "default",
				}
			}
			endpoint.DNSName = dnsName
			endpoint.RecordType = "CNAME"
			endpoint.Targets = []string{dnsNameContinent}
			endpoint.RecordTTL = 60
			endpoint.SetProviderSpecific(aws.ProviderSpecificGeolocationCountryCode, "*")
			newEndpoints = append(newEndpoints, endpoint)
		}

		for _, targets := range hosts {
			for _, target := range targets {
				//We need a geo specific set id
				targetID := fmt.Sprintf("%s.%s", strings.ToLower(continentCode), target)
				// If the endpoint for this target does not exist, add a new one
				if endpoint, ok = currentEndpoints[targetID]; !ok {
					endpoint = &v1.Endpoint{
						SetIdentifier: targetID,
					}
				}

				weight:= awsEndpointWeight(len(hosts))

				r.Log.V(3).Info("setEndpointFromTargets creating A record", "target", target, "dnsNameContinent", dnsNameContinent, "weight", weight)

				// Update the endpoint fields
				endpoint.DNSName = dnsNameContinent
				endpoint.RecordType = "A"
				endpoint.Targets = []string{target}
				endpoint.RecordTTL = 60
				providerSpecific := v1.ProviderSpecific{
					{
						Name:  aws.ProviderSpecificWeight,
						Value: weight,
					},
				}
				endpoint.ProviderSpecific = providerSpecific
				newEndpoints = append(newEndpoints, endpoint)
			}
		}
	}

	sort.Slice(newEndpoints, func(i, j int) bool {
		return newEndpoints[i].Targets[0] < newEndpoints[j].Targets[0]
	})

	r.Log.V(3).Info("setEndpointFromTargets", "newEndpoints", newEndpoints)

	dnsRecord.Spec.Endpoints = newEndpoints
}

// awsEndpointWeight returns the weight Value for a single AWS record in a set of records where the traffic is split
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
// awsGeoContinentCode returns a continent code for the given IP
func awsGeoContinentCode(ip string) string {
	return ipwhois.GetContinentCodeForIp(ip)
}

func objectKey(obj runtime.Object) cache.ExplicitKey {
	key, _ := cache.MetaNamespaceKeyFunc(obj)
	return cache.ExplicitKey(key)
}
