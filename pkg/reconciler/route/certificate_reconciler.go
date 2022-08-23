package route

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	routeapiv1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	"github.com/kcp-dev/logicalcluster/v2"

	"github.com/kuadrant/kcp-glbc/pkg/tls"
)

type certificateReconciler struct {
	createCertificate    func(ctx context.Context, mapper tls.CertificateRequest) error
	deleteCertificate    func(ctx context.Context, mapper tls.CertificateRequest) error
	getCertificateSecret func(ctx context.Context, request tls.CertificateRequest) (*corev1.Secret, error)
	updateCertificate    func(ctx context.Context, request tls.CertificateRequest) error
	getCertificateStatus func(ctx context.Context, request tls.CertificateRequest) (tls.CertStatus, error)
	copySecret           func(ctx context.Context, workspace logicalcluster.Name, namespace string, s *corev1.Secret) error
	getSecret            func(ctx context.Context, workspace logicalcluster.Name, namespace, name string) (*corev1.Secret, error)
	deleteSecret         func(ctx context.Context, workspace logicalcluster.Name, namespace, name string) error
	log                  logr.Logger
}

type enqueue bool

// certificateSecretFilter
func certificateSecretFilter(obj interface{}) bool {
	s, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	if _, ok := s.Labels[LABEL_HCG_MANAGED]; !ok {
		return false
	}
	if s.Annotations != nil {
		if _, ok := s.Annotations[tls.TlsIssuerAnnotation]; ok {
			if _, ok := s.Annotations[annotationIngressKey]; ok {
				return true
			}
		}
	}
	return false
}

//// certificateUpdatedHandler is used as an event handler for certificates
//func certificateUpdatedHandler(oldCert, newCert *certman.Certificate) enqueue {
//	issuer := newCert.Spec.IssuerRef
//
//	revision := func(c *certman.Certificate) int {
//		if c.Status.Revision != nil {
//			return *c.Status.Revision
//		}
//		return 0
//	}
//	// certificate moved from not ready to ready so a new certificate is ready
//	if !certificateReady(oldCert) && certificateReady(newCert) {
//		// if it is the first cert decrement the counter
//		//sometimes we see the new cert move to ready before the revision is incremented. So it can be at revision 0
//		if revision(newCert) == 1 || revision(newCert) == 0 {
//			tlsCertificateRequestCount.WithLabelValues(issuer.Name).Dec()
//			tlsCertificateRequestTotal.WithLabelValues(issuer.Name, resultLabelSucceeded).Inc()
//			tlsCertificateIssuanceDuration.
//				WithLabelValues(issuer.Name, resultLabelSucceeded).
//				Observe(time.Since(newCert.CreationTimestamp.Time).Seconds())
//		}
//		return enqueue(true)
//	}
//
//	var hasFailed = func(cert *certman.Certificate) bool {
//		if cert.Status.LastFailureTime != nil && cert.Status.RenewalTime == nil {
//			return true
//		}
//		if newCert.Status.LastFailureTime != nil && newCert.Status.RenewalTime != nil {
//			// this is a renewal that has failed
//			if newCert.Status.LastFailureTime.Time.After(newCert.Status.RenewalTime.Time) {
//				return true
//			}
//		}
//		return false
//	}
//	// error case
//	if !certificateReady(newCert) {
//		//state transitioned to failure increment counter
//		if hasFailed(newCert) && !hasFailed(oldCert) {
//			tlsCertificateRequestErrors.WithLabelValues(issuer.Name, resultLabelFailed).Inc()
//		}
//	}
//
//	return enqueue(false)
//}
//
//// certificateAddedHandler is used as an event handler for certificates
//func certificateAddedHandler(cert *certman.Certificate) {
//	issuer := cert.Spec.IssuerRef
//	// new cert added so increment
//	tlsCertificateRequestCount.WithLabelValues(issuer.Name).Inc()
//}
//
//// certificateDeletedHandler is used as an event handler
//func certificateDeletedHandler(cert *certman.Certificate) {
//	issuer := cert.Spec.IssuerRef
//	if !certificateReady(cert) {
//		// cert never got to ready but is now being deleted so decerement counter
//		tlsCertificateRequestCount.WithLabelValues(issuer.Name).Dec()
//	}
//}

func certificateReady(cert *certman.Certificate) bool {
	for _, cond := range cert.Status.Conditions {
		if cond.Type == certman.CertificateConditionReady {
			return cond.Status == cmmeta.ConditionTrue
		}
	}
	return false
}

func CertificateName(route *routeapiv1.Route) string {
	// Removes chars which are invalid characters for cert manager certificate names. RFC 1123 subdomain must consist of
	// lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character

	return strings.ReplaceAll(fmt.Sprintf("%s-%s-%s", logicalcluster.From(route), route.Namespace, route.Name), ":", "")
}

// TLSSecretName returns the name for the secret in the end user namespace
func TLSSecretName(route *routeapiv1.Route) string {
	return fmt.Sprintf("hcg-tls-%s", route.Name)
}

func (r *certificateReconciler) reconcile(ctx context.Context, route *routeapiv1.Route) (reconcileStatus, error) {
	ingressAnnotations := route.GetAnnotations()
	if ingressAnnotations == nil {
		ingressAnnotations = map[string]string{}
	}
	annotations := map[string]string{}
	labels := map[string]string{
		LABEL_HCG_MANAGED: "true",
	}
	key, err := cache.MetaNamespaceKeyFunc(route)
	if err != nil {
		return reconcileStatusStop, err
	}
	tlsSecretName := TLSSecretName(route)
	//set the route key on the certificate to help us with locating the route later
	annotations[annotationIngressKey] = key
	certReq := tls.CertificateRequest{
		Name:        CertificateName(route),
		Labels:      labels,
		Annotations: annotations,
		Host:        ingressAnnotations[ANNOTATION_HCG_HOST],
	}

	if route.DeletionTimestamp != nil && !route.DeletionTimestamp.IsZero() {
		if err := r.deleteCertificate(ctx, certReq); err != nil && !strings.Contains(err.Error(), "not found") {
			r.log.Info("error deleting certificate")
			return reconcileStatusStop, err
		}
		//TODO remove once owner refs work in kcp
		if err := r.deleteSecret(ctx, logicalcluster.From(route), route.Namespace, tlsSecretName); err != nil && !strings.Contains(err.Error(), "not found") {
			r.log.Info("error deleting certificate secret")
			return reconcileStatusStop, err
		}
		return reconcileStatusContinue, nil
	}

	err = r.createCertificate(ctx, certReq)
	if errors.IsAlreadyExists(err) {
		// get certificate secret and copy
		secret, err := r.getCertificateSecret(ctx, certReq)
		if err != nil {
			if tls.IsCertNotReadyErr(err) {
				// cetificate not ready so update the status and allow it continue reconcile. Will be requeued once certificate becomes ready
				status, err := r.getCertificateStatus(ctx, certReq)
				if err != nil {
					return reconcileStatusStop, err
				}
				route.Annotations[annotationCertificateState] = string(status)
				return reconcileStatusContinue, nil
			}
			return reconcileStatusStop, err
		}
		route.Annotations[annotationCertificateState] = "ready" // todo remote hardcoded string
		//copy over the secret to the route namesapce
		scopy := secret.DeepCopy()
		scopy.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         routeapiv1.SchemeGroupVersion.String(),
				Kind:               "Route",
				Name:               route.Name,
				UID:                route.UID,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			},
		})

		scopy.Namespace = route.Namespace
		scopy.Name = tlsSecretName
		if err := r.copySecret(ctx, logicalcluster.From(route), route.Namespace, scopy); err != nil {
			return reconcileStatusStop, err
		}
	}
	if err != nil && !errors.IsAlreadyExists(err) {
		return reconcileStatusStop, err
	}
	// set tls setting on the route

	tlsSecret, err := r.getSecret(ctx, logicalcluster.From(route), route.Namespace, tlsSecretName)
	if err != nil {
		return reconcileStatusStop, err
	}

	upsertTLS(route, tlsSecret)

	return reconcileStatusContinue, nil
}

func removeHostsFromTLS(hostsToRemove string, route *routeapiv1.Route) {
	//for _, host := range hostsToRemove {
	//	for i, tls := range route.Spec.TLS {
	//		hosts := tls.Hosts
	//		for j, ingressHost := range tls.Hosts {
	//			if ingressHost == host {
	//				hosts = append(hosts[:j], hosts[j+1:]...)
	//			}
	//		}
	//		// if there are no hosts remaining remove the entry for TLS
	//		if len(hosts) == 0 {
	//			route.Spec.TLS = append(route.Spec.TLS[:i], route.Spec.TLS[i+1:]...)
	//		} else {
	//			route.Spec.TLS[i].Hosts = hosts
	//		}
	//	}
	//}
}

func upsertTLS(route *routeapiv1.Route, tlsSecret *corev1.Secret) {
	caCrt := tlsSecret.Data["ca.crt"]
	tlsCrt := tlsSecret.Data["tls.crt"]
	tlsKey := tlsSecret.Data["tls.key"]

	tlsConfig := &routeapiv1.TLSConfig{}
	tlsConfig.Termination = routeapiv1.TLSTerminationEdge
	tlsConfig.Key = string(tlsKey)
	tlsConfig.Certificate = string(tlsCrt)
	tlsConfig.CACertificate = string(caCrt)

	route.Spec.TLS = tlsConfig
}

func (c *Controller) getTLSSecret(ctx context.Context, workspace logicalcluster.Name, namespace, name string) (*corev1.Secret, error) {
	secret, err := c.kubeClient.Cluster(workspace).CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	return secret, nil
}

func (c *Controller) deleteTLSSecret(ctx context.Context, workspace logicalcluster.Name, namespace, name string) error {
	if err := c.kubeClient.Cluster(workspace).CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) copySecret(ctx context.Context, workspace logicalcluster.Name, namespace string, secret *corev1.Secret) error {
	secret.ResourceVersion = ""
	secretClient := c.kubeClient.Cluster(workspace).CoreV1().Secrets(namespace)
	_, err := secretClient.Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && k8serrors.IsAlreadyExists(err) {
		s, err := secretClient.Get(ctx, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		s.Data = secret.Data
		if _, err := secretClient.Update(ctx, s, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	return nil

}
