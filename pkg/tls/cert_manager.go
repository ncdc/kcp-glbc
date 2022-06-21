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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"os"
	"strings"
	"time"

	cmacme "github.com/jetstack/cert-manager/pkg/apis/acme/v1"
	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	certmanclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DNSValidator int

const (
	DNSValidatorRoute53  DNSValidator = iota
	DefaultCertificateNS string       = "cert-manager"
)

type CertProvider string

const (
	CertProviderCA        CertProvider = "glbc-ca"
	CertProviderLEStaging CertProvider = "letsencryptstaging"
	CertProviderLEProd    CertProvider = "letsencryptprod"

	caSecretName string = "glbc-ca"

	awsSecretName      string = "route53-credentials"
	envAwsAccessKeyID  string = "AWS_ACCESS_KEY_ID"
	envAwsAccessSecret string = "AWS_SECRET_ACCESS_KEY"
	envAwsZoneID       string = "AWS_DNS_PUBLIC_ZONE_ID"

	envLEEmail   string = "HCG_LE_EMAIL"
	leProdAPI    string = "https://acme-v02.api.letsencrypt.org/directory"
	leStagingAPI string = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

// certManager is a certificate provider.
type certManager struct {
	dnsValidationProvider DNSValidator
	certClient            certmanclient.CertmanagerV1Interface
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

type CertManagerConfig struct {
	DNSValidator DNSValidator
	CertClient   certmanclient.CertmanagerV1Interface

	CertProvider CertProvider
	LEConfig     *LEConfig
	Region       string
	// client targeting the control cluster
	K8sClient kubernetes.Interface
	// namespace in the control cluster where we create certificates
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

	switch cm.certProvider {

	case CertProviderLEStaging, CertProviderLEProd:
		if os.Getenv(envAwsAccessKeyID) == "" || os.Getenv(envAwsAccessSecret) == "" || os.Getenv(envAwsZoneID) == "" {
			return nil, fmt.Errorf(fmt.Sprintf("certmanager is missing envars for aws %s %s %s", envAwsAccessKeyID, envAwsAccessSecret, envAwsZoneID))
		}
		if os.Getenv(envLEEmail) == "" {
			return nil, fmt.Errorf("certmanager: missing env var %s", envLEEmail)
		}
	}

	return cm, nil
}

func (cm *certManager) IssuerID() string {
	return string(cm.certProvider)
}

func (cm *certManager) Domains() []string {
	return cm.validDomains
}

// Initialize will configure the issuer and aws access secret.
// TODO this should probably be in a controller for a GLBC CRD, or externalized in Issuer resources altogether
func (cm *certManager) Initialize(ctx context.Context) error {
	switch cm.certProvider {

	case CertProviderLEStaging, CertProviderLEProd:
		return cm.createIssuerLE(ctx)

	case CertProviderCA:
		return cm.createIssuerCA(ctx)

	default:
		return fmt.Errorf("unsupported TLS certificate provider %q", cm.certProvider)
	}
}

func (cm *certManager) Create(ctx context.Context, cr CertificateRequest) error {
	if !isValidDomain(cr.Host(), cm.validDomains) {
		return fmt.Errorf("cannot create certificate for host %s invalid domain", cr.Host())
	}
	cert := cm.certificate(cr)
	_, err := cm.certClient.Certificates(cm.certificateNS).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// TODO: Move to Certificate informer add handler
	CertificateRequestCount.WithLabelValues(cm.IssuerID()).Inc()
	return nil
}

func (cm *certManager) Delete(ctx context.Context, cr CertificateRequest) error {
	// delete the certificate and delete the secrets
	certNotFound := false

	if err := cm.certClient.Certificates(cm.certificateNS).Delete(ctx, cr.Name(), metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		certNotFound = true
	}
	if err := cm.k8sClient.CoreV1().Secrets(cm.certificateNS).Delete(ctx, cr.Name(), metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if !certNotFound {
			// The Secret does not exist, which indicates the TLS certificate request is still pending,
			// so we must account for decreasing the number of pending requests.
			// TODO: Move to Certificate informer delete handler
			CertificateRequestCount.WithLabelValues(cm.IssuerID()).Dec()
		}
	}
	return nil
}

func (cm *certManager) certificate(cr CertificateRequest) *certman.Certificate {
	annotations := cr.Annotations()
	annotations[tlsIssuerAnnotation] = cm.IssuerID()
	return &certman.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name(),
			Namespace: cm.certificateNS,
		},
		Spec: certman.CertificateSpec{
			SecretName: cr.Name(),
			SecretTemplate: &certman.CertificateSecretTemplate{
				Labels:      cr.Labels(),
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
			DNSNames: []string{cr.Host()},
			IssuerRef: cmmeta.ObjectReference{
				Group: "cert-manager.io",
				Kind:  "Issuer",
				Name:  string(cm.certProvider),
			},
		},
	}
}

func isValidDomain(host string, allowed []string) bool {
	for _, v := range allowed {
		if strings.HasSuffix(host, v) {
			return true
		}
	}
	return false
}

func (cm *certManager) createIssuer(ctx context.Context, secret *corev1.Secret, issuer *certman.Issuer) error {
	_, err := cm.k8sClient.CoreV1().Secrets(cm.certificateNS).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	if err != nil {
		s, err := cm.k8sClient.CoreV1().Secrets(cm.certificateNS).Get(ctx, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		secret.ResourceVersion = s.ResourceVersion
		_, err = cm.k8sClient.CoreV1().Secrets(cm.certificateNS).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	_, err = cm.certClient.Issuers(cm.certificateNS).Create(ctx, issuer, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	if err != nil {
		is, err := cm.certClient.Issuers(cm.certificateNS).Get(ctx, issuer.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		issuer.ResourceVersion = is.ResourceVersion
		if _, err := cm.certClient.Issuers(cm.certificateNS).Update(ctx, issuer, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (cm *certManager) createIssuerLE(ctx context.Context) error {
	return cm.createIssuer(ctx, awsSecret(), cm.issuerLE())
}

func (cm *certManager) issuerLE() *certman.Issuer {
	server := leStagingAPI
	if cm.certProvider == CertProviderLEProd {
		server = leProdAPI
	}

	return &certman.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cm.certificateNS,
			Name:      string(cm.certProvider),
		},
		Spec: certman.IssuerSpec{
			IssuerConfig: certman.IssuerConfig{
				ACME: &cmacme.ACMEIssuer{
					Email:  os.Getenv(envLEEmail),
					Server: server,
					PrivateKey: cmmeta.SecretKeySelector{
						LocalObjectReference: cmmeta.LocalObjectReference{
							Name: string(cm.certProvider),
						},
					},
					Solvers: []cmacme.ACMEChallengeSolver{
						{
							DNS01: &cmacme.ACMEChallengeSolverDNS01{
								Route53: &cmacme.ACMEIssuerDNS01ProviderRoute53{
									AccessKeyID:  os.Getenv(envAwsAccessKeyID),
									HostedZoneID: os.Getenv(envAwsZoneID),
									Region:       cm.Region,
									SecretAccessKey: cmmeta.SecretKeySelector{
										LocalObjectReference: cmmeta.LocalObjectReference{
											Name: awsSecretName,
										},
										Key: envAwsAccessSecret,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func awsSecret() *corev1.Secret {
	accessKeyID := os.Getenv(envAwsAccessKeyID)
	accessSecret := os.Getenv(envAwsAccessSecret)
	zoneID := os.Getenv(envAwsZoneID)

	data := make(map[string][]byte)

	data[envAwsAccessKeyID] = []byte(accessKeyID)
	data[envAwsAccessSecret] = []byte(accessSecret)
	data[envAwsZoneID] = []byte(zoneID)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: awsSecretName,
		},
		Data: data,
	}
}

func (cm *certManager) createIssuerCA(ctx context.Context) error {
	if secret, err := cm.caSecret(); err != nil {
		return err
	} else {
		return cm.createIssuer(ctx, secret, cm.issuerCA())
	}
}

func (cm *certManager) issuerCA() *certman.Issuer {
	return &certman.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cm.certificateNS,
			Name:      string(cm.certProvider),
		},
		Spec: certman.IssuerSpec{
			IssuerConfig: certman.IssuerConfig{
				CA: &certman.CAIssuer{
					SecretName: caSecretName,
				},
			},
		},
	}
}

func (cm *certManager) caSecret() (*corev1.Secret, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(mathrand.Int63()),
		Subject: pkix.Name{
			Organization: []string{"Kuadrant"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, err
	}

	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	privateKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	})

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: caSecretName,
		}, Data: map[string][]byte{
			corev1.TLSCertKey:       certPem,
			corev1.TLSPrivateKeyKey: privateKeyPem,
		}, Type: corev1.SecretTypeTLS,
	}, nil
}
