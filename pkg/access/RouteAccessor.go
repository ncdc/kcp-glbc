package access

import (
	"context"

	"github.com/kcp-dev/logicalcluster/v2"
	routev1 "github.com/openshift/api/route/v1"

	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/reconciler/dns"
)

//RouteAccessor - placeholder, not yet implemented
type RouteAccessor struct {
	object *routev1.Route
}

func NewRouteAccessor(r *routev1.Route) *RouteAccessor {
	return &RouteAccessor{object: r}
}

func (a *RouteAccessor) GetKind() string {
	return "Route"
}

func (a *RouteAccessor) GetHosts() []string {
	return []string{}
}

func (a *RouteAccessor) AddTLS(host, secret string) {
}

func (a *RouteAccessor) RemoveTLS(hosts []string) {
}

func (a *RouteAccessor) ReplaceCustomHosts(managedHost string) []string {
	return []string{}
}

func (a *RouteAccessor) GetTargets(ctx context.Context, dnsLookup dnsLookupFunc) (map[logicalcluster.Name]map[string]dns.Target, error) {
	return map[logicalcluster.Name]map[string]dns.Target{}, nil
}

func (a *RouteAccessor) ProcessCustomHosts(dvs *v1.DomainVerificationList) error {
	return nil
}

func (a *RouteAccessor) String() string {
	return ""
}
