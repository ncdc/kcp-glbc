package route

import (
	"context"
	"strconv"
	"strings"

	"github.com/kuadrant/kcp-glbc/pkg/util/workloadMigration"
	routeapiv1 "github.com/openshift/api/route/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"
	utilserrors "k8s.io/apimachinery/pkg/util/errors"
)

type reconcileStatus int

const (
	reconcileStatusStop reconcileStatus = iota
	reconcileStatusContinue
	cascadeCleanupFinalizer = "kcp.dev/cascade-cleanup"
)

type reconciler interface {
	reconcile(ctx context.Context, route *routeapiv1.Route) (reconcileStatus, error)
}

func (c *Controller) reconcile(ctx context.Context, route *routeapiv1.Route) error {
	c.Logger.V(3).Info("starting reconcile of route ", route.Name, route.Namespace)
	if route.DeletionTimestamp == nil {
		metadata.AddFinalizer(route, cascadeCleanupFinalizer)
	}
	//TODO evaluate where this actually belongs
	workloadMigration.Process(route, c.Queue, c.Logger)

	reconcilers := []reconciler{
		//hostReconciler is first as the others depends on it for the host to be set on the route
		&hostReconciler{
			managedDomain: c.domain,
			log:           c.Logger,
		},
		&certificateReconciler{
			createCertificate:    c.certProvider.Create,
			deleteCertificate:    c.certProvider.Delete,
			getCertificateSecret: c.certProvider.GetCertificateSecret,
			updateCertificate:    c.certProvider.Update,
			getCertificateStatus: c.certProvider.GetCertificateStatus,
			copySecret:           c.copySecret,
			deleteSecret:         c.deleteTLSSecret,
			getSecret:            c.getTLSSecret,
			log:                  c.Logger,
		},
		&dnsReconciler{
			deleteDNS:        c.deleteDNS,
			DNSLookup:        c.hostResolver.LookupIPAddr,
			getDNS:           c.getDNS,
			createDNS:        c.createDNS,
			updateDNS:        c.updateDNS,
			watchHost:        c.hostsWatcher.StartWatching,
			forgetHost:       c.hostsWatcher.StopWatching,
			listHostWatchers: c.hostsWatcher.ListHostRecordWatchers,
			log:              c.Logger,
		},
	}
	var errs []error

	for _, r := range reconcilers {
		status, err := r.reconcile(ctx, route)
		if err != nil {
			errs = append(errs, err)
		}
		if status == reconcileStatusStop {
			break
		}
	}

	if len(errs) == 0 {
		if route.DeletionTimestamp != nil && !route.DeletionTimestamp.IsZero() {
			metadata.RemoveFinalizer(route, cascadeCleanupFinalizer)
			c.hostsWatcher.StopWatching(routeKey(route), "")
			//in 0.5.0 these are never cleaned up properly
			for _, f := range route.Finalizers {
				if strings.Contains(f, workloadMigration.SyncerFinalizer) {
					metadata.RemoveFinalizer(route, f)
				}
			}
		}
	}
	c.Logger.V(3).Info("route reconcile complete", "reconciler errors", strconv.Itoa(len(errs)), "namespace", route.Namespace, "resource name", route.Name)
	return utilserrors.NewAggregate(errs)
}

func routeKey(route *routeapiv1.Route) interface{} {
	key, _ := cache.MetaNamespaceKeyFunc(route)
	return cache.ExplicitKey(key)
}
