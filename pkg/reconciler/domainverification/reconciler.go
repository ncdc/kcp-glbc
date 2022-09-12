package domainverification

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilserrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/kcp-dev/logicalcluster/v2"

	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/net"
)

type reconcileStatus int

const (
	reconcileStatusStop reconcileStatus = iota
	reconcileStatusContinue
)

type reconciler interface {
	reconcile(ctx context.Context, dv *v1.DomainVerification) (reconcileStatus, error)
	Name() string
}

type DNSVerifier interface {
	TxtRecordExists(ctx context.Context, domain string, value string) (bool, error)
}

type domainVerificationStatus struct {
	dnsVerifier DNSVerifier
	requeAfter  func(item interface{}, duration time.Duration)
	name        string
}

func (dsr *domainVerificationStatus) Name() string {
	return dsr.name
}

// reconcile ensures the status is as expected
func (dsr *domainVerificationStatus) reconcile(ctx context.Context, dv *v1.DomainVerification) (reconcileStatus, error) {
	var status = reconcileStatusContinue
	var errs error
	verified, ensureErr := dsr.ensureDomainVerificationStatus(ctx, dv)
	if ensureErr != nil && !net.IsNoSuchHostError(ensureErr) {
		errs = multierror.Append(errs, fmt.Errorf("error ensuring domain verification: %v", ensureErr))
		status = reconcileStatusStop
	} else if ensureErr != nil && net.IsNoSuchHostError(ensureErr) {
		//don't return error if host does not exist, returning errors here causes an immediate requeue of the resource
		status = reconcileStatusStop
	}

	if !verified {
		status = reconcileStatusStop
		dsr.requeAfter(dv, recheckDefault)
	}

	return status, errs
}
func (dsr *domainVerificationStatus) ensureDomainVerificationStatus(ctx context.Context, domainVerification *v1.DomainVerification) (bool, error) {
	// default status
	domainVerification.Status.Verified = false

	if domainVerification.Status.Token == "" {
		domainVerification.Status.Token = domainVerification.GetToken()
		return false, nil
	}

	// check if this domain is already verified. Trusting the webhook to ensure this is only updated by our controller
	if domainVerification.Status.Verified {
		return true, nil
	}
	domainVerification.Status.LastChecked = metav1.Now()
	// check DNS to see can we validate
	exists, err := dsr.dnsVerifier.TxtRecordExists(ctx, domainVerification.Spec.Domain, domainVerification.Status.Token)
	if err != nil {
		domainVerification.Status.Message = fmt.Sprintf("domain verification was not successful: %v", err)
		domainVerification.Status.NextCheck = metav1.NewTime(time.Now().Add(recheckDefault))
		return false, err
	} else if !exists {
		domainVerification.Status.Message = "domain verification was not successful: TXT record does not exist"
		domainVerification.Status.NextCheck = metav1.NewTime(time.Now().Add(recheckDefault))
		return false, nil
	}
	domainVerification.Status.Message = "domain verification was successful"
	domainVerification.Status.Verified = true

	return exists, nil
}

func (c *Controller) reconcile(ctx context.Context, domainVerification *v1.DomainVerification) error {
	c.Logger.V(3).Info("starting reconcile of domainVerification ", "name", domainVerification.Name, "namespace", domainVerification.Namespace, "cluster", logicalcluster.From(domainVerification))
	reconcilers := []reconciler{
		&domainVerificationStatus{
			dnsVerifier: c.dnsVerifier,
			requeAfter:  c.EnqueueAfter,
			name:        "domainVerificationStatus",
		},
	}

	var errs []error

	for _, r := range reconcilers {
		status, err := r.reconcile(ctx, domainVerification)
		if err != nil {
			errs = append(errs, fmt.Errorf("error from reconciler '%v': %v", r.Name(), err))
		}
		if status == reconcileStatusStop {
			break
		}
	}
	return utilserrors.NewAggregate(errs)
}
