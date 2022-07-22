package host

import (
	"fmt"

	"github.com/rs/xid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ANNOTATION_HCG_HOST = "kuadrant.dev/host.generated"

type Hosts interface {
	SetManagedHost(obj metav1.Object)
	GetManagedHost(o metav1.Object) (string, error)
	ManagedHostSet(o metav1.Object) bool
}

type service struct {
	managedDomain string
}

func NewService(managedDomain string) *service {
	return &service{managedDomain: managedDomain}
}

func (h *service) SetManagedHost(o metav1.Object) {
	generatedHost := fmt.Sprintf("%s.%s", xid.New(), h.managedDomain)
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[ANNOTATION_HCG_HOST] = generatedHost
	o.SetAnnotations(annotations)
}

func (h *service) GetManagedHost(o metav1.Object) (string, error) {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return "", fmt.Errorf("no managed host set")
	}
	host, ok := annotations[ANNOTATION_HCG_HOST]
	if !ok {
		return "", fmt.Errorf("no managed host set")
	}
	return host, nil
}

func (h *service) ManagedHostSet(o metav1.Object) bool {
	annotations := o.GetAnnotations()
	_, ok := annotations[ANNOTATION_HCG_HOST]
	return ok
}
