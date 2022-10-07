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

package support

import (
	"crypto/x509"
	"encoding/pem"
	"errors"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CertificateFrom(secret *corev1.Secret) (*x509.Certificate, error) {
	certBytes := secret.Data["tls.crt"]
	pemBlock, _ := pem.Decode(certBytes)
	if pemBlock == nil {
		return nil, errors.New("failed to decode certificate")
	}
	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, err
}

func GetCertificate(t Test, namespace, name string) *certman.Certificate {
	t.T().Helper()
	return TLSCertificate(t, namespace, name)(t)
}

func TLSCertificate(t Test, namespace, name string) func(g gomega.Gomega) *certman.Certificate {
	return func(g gomega.Gomega) *certman.Certificate {
		cert, err := t.Client().Certs().CertmanagerV1().Certificates(namespace).Get(t.Ctx(), name, v1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return cert
	}
}

func TLSCertificateSpec(cert *certman.Certificate) certman.CertificateSpec {
	return cert.Spec
}
