/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tls

import (
	"context"
	"fmt"
	"strings"
	"time"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	certmanclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DNSValidator int

const (
	DNSValidatorRoute53  DNSValidator = iota
	DefaultCertificateNS string       = "kcp-glbc"
	certFinalizer                     = "kuadrant.dev/certificates-cleanup"
)

type CertProvider string

// certManager is a certificate provider.
type certManager struct {
	dnsValidationProvider DNSValidator
	certClient            certmanclient.Interface
	k8sClient             kubernetes.Interface
	certProvider          CertProvider
	LEConfig              LEConfig
	Region                string
	certificateNS         string
	validDomains          []string
}

var _ Provider = &certManager{}

type LEConfig struct {
	Email string
}

var CertNotReadyErr = fmt.Errorf("certificate is not ready yet ")

func IsCertNotReadyErr(err error) bool {
	return err == CertNotReadyErr
}

type CertManagerConfig struct {
	DNSValidator DNSValidator
	CertClient   certmanclient.Interface

	CertProvider CertProvider
	LEConfig     *LEConfig
	Region       string
	// client targeting the glbc workspace cluster
	K8sClient kubernetes.Interface
	// namespace in the control workspace where we create certificates
	CertificateNS string
	// set of domains we allow certs to be created for
	ValidDomains []string
}

func NewCertManager(c CertManagerConfig) (*certManager, error) {
	cm := &certManager{
		dnsValidationProvider: c.DNSValidator,
		certClient:            c.CertClient,
		k8sClient:             c.K8sClient,
		certProvider:          c.CertProvider,
		Region:                c.Region,
		validDomains:          c.ValidDomains,
		certificateNS:         c.CertificateNS,
	}

	return cm, nil
}

func (cm *certManager) IssuerID() string {
	return string(cm.certProvider)
}

func (cm *certManager) Domains() []string {
	return cm.validDomains
}

func (cm *certManager) IssuerExists(ctx context.Context) (bool, error) {
	_, err := cm.certClient.CertmanagerV1().Issuers(cm.certificateNS).Get(ctx, cm.IssuerID(), metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (cm *certManager) GetCertificateSecret(ctx context.Context, request CertificateRequest) (*corev1.Secret, error) {
	c, err := cm.certClient.CertmanagerV1().Certificates(cm.certificateNS).Get(ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, cond := range c.Status.Conditions {
		if cond.Type == certman.CertificateConditionReady && cond.Status != cmmeta.ConditionTrue {
			return nil, CertNotReadyErr
		}
	}
	return cm.k8sClient.CoreV1().Secrets(cm.certificateNS).Get(ctx, c.Spec.SecretName, metav1.GetOptions{})

}

func (cm *certManager) GetCertificate(ctx context.Context, certReq CertificateRequest) (*certman.Certificate, error) {
	return cm.certClient.CertmanagerV1().Certificates(cm.certificateNS).Get(ctx, certReq.Name, metav1.GetOptions{})
}

func (cm *certManager) GetCertificateStatus(ctx context.Context, certReq CertificateRequest) (CertStatus, error) {
	cert, err := cm.GetCertificate(ctx, certReq)
	if err != nil {
		return CertStatus("unknown"), err
	}
	for _, cond := range cert.Status.Conditions {
		if cond.Type == certman.CertificateConditionIssuing && cond.Status == cmmeta.ConditionTrue {
			return CertStatus("issuing"), nil
		}
		if cond.Type == certman.CertificateConditionReady && cond.Status == cmmeta.ConditionTrue {
			return CertStatus("ready"), nil
		}
	}
	// TODO look into identifying an error status. There is a lastFailureTime
	return CertStatus("unknown"), err
}

func (cm *certManager) Create(ctx context.Context, cr CertificateRequest) error {
	if !isValidDomain(cr.Host, cm.validDomains) {
		return fmt.Errorf("cannot create certificate for host %s invalid domain", cr.Host)
	}
	cert := cm.certificate(cr)
	// add finalizer
	metadata.AddFinalizer(cert, certFinalizer)
	_, err := cm.certClient.CertmanagerV1().Certificates(cm.certificateNS).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (cm *certManager) Delete(ctx context.Context, cr CertificateRequest) error {
	// delete the certificate and delete the secrets
	// remove finalizer (todo come up with better way of handlng this)
	cr.cleanUpFinalizer = true
	if err := cm.Update(ctx, cr); err != nil {
		return err
	}
	if err := cm.certClient.CertmanagerV1().Certificates(cm.certificateNS).Delete(ctx, cr.Name, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	if err := cm.k8sClient.CoreV1().Secrets(cm.certificateNS).Delete(ctx, cr.Name, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (cm *certManager) certificate(cr CertificateRequest) *certman.Certificate {
	annotations := cr.Annotations
	annotations[TlsIssuerAnnotation] = cm.IssuerID()
	labels := cr.Labels
	return &certman.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Namespace:   cm.certificateNS,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: certman.CertificateSpec{
			SecretName: cr.Name,
			SecretTemplate: &certman.CertificateSecretTemplate{
				Labels:      labels,
				Annotations: annotations,
			},
			// TODO Some of the below should be pulled out into a CRD
			Duration: &metav1.Duration{
				Duration: time.Hour * 24 * 90, // cert lasts for 90 days
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Hour * 24 * 15, // cert is renewed 15 days before hand
			},
			PrivateKey: &certman.CertificatePrivateKey{
				Algorithm: certman.RSAKeyAlgorithm,
				Encoding:  certman.PKCS1,
				Size:      2048,
			},
			Usages:   certman.DefaultKeyUsages(),
			DNSNames: []string{cr.Host},
			IssuerRef: cmmeta.ObjectReference{
				Group: "cert-manager.io",
				Kind:  "Issuer",
				Name:  string(cm.certProvider),
			},
		},
	}
}

func (cm *certManager) Update(ctx context.Context, cr CertificateRequest) error {
	cert, err := cm.GetCertificate(ctx, cr)
	if err != nil {
		return err
	}
	if cert.Labels == nil {
		cert.Labels = map[string]string{}
	}
	if cert.Annotations == nil {
		cert.Annotations = map[string]string{}
	}
	for k, v := range cr.Labels {
		cert.Labels[k] = v
	}
	for k, v := range cr.Annotations {
		cert.Annotations[k] = v
	}
	if cr.cleanUpFinalizer {
		metadata.RemoveFinalizer(cert, certFinalizer)
	}
	if _, err := cm.certClient.CertmanagerV1().Certificates(cm.certificateNS).Update(ctx, cert, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

func isValidDomain(host string, allowed []string) bool {
	for _, v := range allowed {
		if strings.HasSuffix(host, v) {
			return true
		}
	}
	return false
}
