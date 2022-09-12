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

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	v1 "k8s.io/api/core/v1"
)

const TlsIssuerAnnotation = "kuadrant.dev/tls-issuer"

type Provider interface {
	IssuerID() string
	Domains() []string
	Create(ctx context.Context, cr CertificateRequest) error
	Delete(ctx context.Context, cr CertificateRequest) error
	Update(ctx context.Context, cr CertificateRequest) error
	GetCertificateSecret(ctx context.Context, cr CertificateRequest) (*v1.Secret, error)
	GetCertificate(ctx context.Context, cr CertificateRequest) (*certman.Certificate, error)
	GetCertificateStatus(ctx context.Context, certReq CertificateRequest) (CertStatus, error)
	IssuerExists(ctx context.Context) (bool, error)
}

type CertificateRequest struct {
	Name             string
	Labels           map[string]string
	Annotations      map[string]string
	Host             string
	cleanUpFinalizer bool
}

type CertStatus string
