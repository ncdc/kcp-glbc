package ingress

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kuadrant/kcp-glbc/pkg/host"
	networkingv1 "k8s.io/api/networking/v1"
)

type hostReconciler struct {
	log   logr.Logger
	hosts host.Hosts
}

func (r *hostReconciler) reconcile(ctx context.Context, ingress *networkingv1.Ingress) (reconcileStatus, error) {
	if !r.hosts.ManagedHostSet(ingress) {
		// Let's assign it a global hostname if any
		r.hosts.SetManagedHost(ingress)
		//we need this host set and saved on the ingress before we go any further so force an update
		// if this is not saved we end up with a new host and the certificate can have the wrong host
		return reconcileStatusStop, nil
	}
	//once the annotation is definintely saved continue on
	managedHost, err := r.hosts.GetManagedHost(ingress)
	if err != nil {
		return reconcileStatusStop, err
	}
	// custom logic for ingress type only
	var customHosts []string
	for i, rule := range ingress.Spec.Rules {
		if rule.Host != managedHost {
			ingress.Spec.Rules[i].Host = managedHost
			customHosts = append(customHosts, rule.Host)
		}
	}
	removeHostsFromTLS(customHosts, ingress)
	if len(customHosts) > 0 {
		ingress.Annotations[ANNOTATION_HCG_CUSTOM_HOST_REPLACED] = fmt.Sprintf(" replaced custom hosts %v to the glbc host due to custom host policy not being allowed",
			customHosts)
	}

	return reconcileStatusContinue, nil
}
