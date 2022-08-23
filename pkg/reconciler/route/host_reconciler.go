package route

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	routeapiv1 "github.com/openshift/api/route/v1"
	"github.com/rs/xid"
)

type hostReconciler struct {
	managedDomain string
	log           logr.Logger
}

func (r *hostReconciler) reconcile(ctx context.Context, route *routeapiv1.Route) (reconcileStatus, error) {
	r.log.V(3).Info("host reconciler start")
	if route.Annotations == nil || route.Annotations[ANNOTATION_HCG_HOST] == "" {

		// Let's assign it a global hostname if any
		generatedHost := fmt.Sprintf("%s.%s", xid.New(), r.managedDomain)
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}
		route.Annotations[ANNOTATION_HCG_HOST] = generatedHost
		//we need this host set and saved on the route before we go any further so force an update
		// if this is not saved we end up with a new host and the certificate can have the wrong host
		return reconcileStatusStop, nil
	}
	//ToDo What to do here?
	//once the annotation is definitely saved continue on
	var customHost string
	managedHost := route.Annotations[ANNOTATION_HCG_HOST]
	if route.Spec.Host != managedHost {
		customHost = route.Spec.Host
		route.Spec.Host = managedHost
		route.Annotations[ANNOTATION_HCG_CUSTOM_HOST_REPLACED] = customHost
	}
	//clean up replaced hosts from the tls list
	removeHostsFromTLS(customHost, route)

	r.log.V(3).Info("host reconciler complete")
	return reconcileStatusContinue, nil
}
