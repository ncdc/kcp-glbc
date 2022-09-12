package reconcilers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/rs/xid"

	"github.com/kuadrant/kcp-glbc/pkg/access"
	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	kuadrantclientv1 "github.com/kuadrant/kcp-glbc/pkg/client/kuadrant/clientset/versioned"
	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"
)

type HostReconciler struct {
	ManagedDomain          string
	Log                    logr.Logger
	CustomHostsEnabled     bool
	KuadrantClient         kuadrantclientv1.ClusterInterface
	GetDomainVerifications func(ctx context.Context, accessor access.Accessor) (*v1.DomainVerificationList, error)
}

func (r *HostReconciler) Reconcile(ctx context.Context, accessor access.Accessor) (access.ReconcileStatus, error) {
	if !metadata.HasAnnotation(accessor, access.ANNOTATION_HCG_HOST) {
		// Let's assign it a global hostname if any
		generatedHost := fmt.Sprintf("%s.%s", xid.New(), r.ManagedDomain)
		metadata.AddAnnotation(accessor, access.ANNOTATION_HCG_HOST, generatedHost)
		//we need this host set and saved on the accessor before we go any further so force an update
		// if this is not saved we end up with a new host and the certificate can have the wrong host
		return access.ReconcileStatusStop, nil
	}
	if !r.CustomHostsEnabled {
		hcgHost := accessor.GetAnnotations()[access.ANNOTATION_HCG_HOST]
		replacedHosts := accessor.ReplaceCustomHosts(hcgHost)
		if len(replacedHosts) > 0 {
			metadata.AddAnnotation(accessor, access.ANNOTATION_HCG_CUSTOM_HOST_REPLACED, fmt.Sprintf(" replaced custom hosts %v to the glbc host due to custom host policy not being allowed", replacedHosts))
		}
		return access.ReconcileStatusContinue, nil
	}
	return r.processCustomHosts(ctx, accessor)
}

func (r *HostReconciler) processCustomHosts(ctx context.Context, accessor access.Accessor) (access.ReconcileStatus, error) {
	dvs, err := r.GetDomainVerifications(ctx, accessor)
	if err != nil {
		return access.ReconcileStatusContinue, err
	}
	err = accessor.ProcessCustomHosts(dvs)
	if err != nil {
		return access.ReconcileStatusStop, err
	}
	return access.ReconcileStatusContinue, nil
}
