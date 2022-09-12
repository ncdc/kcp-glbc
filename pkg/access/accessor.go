package access

import (
	"context"
	"strings"

	"github.com/kcp-dev/logicalcluster/v2"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/reconciler/dns"
)

type ReconcileStatus int

const (
	ReconcileStatusStop ReconcileStatus = iota
	ReconcileStatusContinue

	ANNOTATION_INGRESS_KEY              = "kuadrant.dev/ingress-key"
	ANNOTATION_CERTIFICATE_STATE        = "kuadrant.dev/certificate-status"
	ANNOTATION_HCG_HOST                 = "kuadrant.dev/host.generated"
	ANNOTATION_HEALTH_CHECK_PREFIX      = "kuadrant.experimental/health-"
	ANNOTATION_HCG_CUSTOM_HOST_REPLACED = "kuadrant.dev/custom-hosts.replaced"
	ANNOTATION_PENDING_CUSTOM_HOSTS     = "kuadrant.dev/pendingCustomHosts"
	LABEL_HAS_PENDING_HOSTS             = "kuadrant.dev/hasPendingCustomHosts"
)

type Accessor interface {
	runtime.Object
	metav1.Object
	GetKind() string
	GetHosts() []string
	AddTLS(host, secret string)
	RemoveTLS(host []string)
	GetTargets(ctx context.Context, dnsLookup dnsLookupFunc) (map[logicalcluster.Name]map[string]dns.Target, error)
	ReplaceCustomHosts(managedHost string) []string
	ProcessCustomHosts(dvs *v1.DomainVerificationList) error
}

type Pending struct {
	Rules []networkingv1.IngressRule `json:"rules"`
}

// IsDomainVerified will take the host and recursively remove subdomains searching for a matching domainverification
// that is verified. Until either a match is found, or the subdomains run out.
func IsDomainVerified(host string, dvs []v1.DomainVerification) bool {
	for _, dv := range dvs {
		if dv.Spec.Domain == host && dv.Status.Verified {
			return true
		}
	}
	parentHostParts := strings.SplitN(host, ".", 2)
	//we've run out of sub-domains
	if len(parentHostParts) < 2 {
		return false
	}

	//recurse up the subdomains
	return IsDomainVerified(parentHostParts[1], dvs)
}
