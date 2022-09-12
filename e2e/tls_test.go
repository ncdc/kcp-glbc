//go:build e2e

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

package e2e

import (
	"crypto/x509/pkix"
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	apisv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apis/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v2"

	. "github.com/kuadrant/kcp-glbc/e2e/support"
	"github.com/kuadrant/kcp-glbc/pkg/access"
	"github.com/kuadrant/kcp-glbc/pkg/access/reconcilers"
	kuadrantv1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/util/env"
)

func TestTLS(t *testing.T) {
	test := With(t)
	test.T().Parallel()

	// Create the test workspace
	workspace := test.NewTestWorkspace()

	// Import compute workspace APIs
	binding := test.NewAPIBinding("kubernetes", WithComputeServiceExport(GLBCWorkspace), InWorkspace(workspace))

	// Wait until the APIBinding is actually in bound phase
	test.Eventually(APIBinding(test, logicalcluster.From(binding).String(), binding.Name)).
		Should(WithTransform(APIBindingPhase, Equal(apisv1alpha1.APIBindingPhaseBound)))

	// Wait until the APIs are imported into the test workspace
	test.Eventually(HasImportedAPIs(test, workspace,
		corev1.SchemeGroupVersion.WithKind("Service"),
		appsv1.SchemeGroupVersion.WithKind("Deployment"),
		networkingv1.SchemeGroupVersion.WithKind("Ingress"),
	)).Should(BeTrue())

	binding = GetAPIBinding(test, logicalcluster.From(binding).String(), binding.Name)
	kubeIdentityHash := binding.Status.BoundResources[0].Schema.IdentityHash

	// Import GLBC APIs
	binding = test.NewAPIBinding("glbc", WithExportReference(GLBCWorkspace, GLBCExportName), WithGLBCAcceptablePermissionClaims(kubeIdentityHash), InWorkspace(workspace))

	// Wait until the APIBinding is actually in bound phase
	test.Eventually(APIBinding(test, logicalcluster.From(binding).String(), binding.Name)).
		Should(WithTransform(APIBindingPhase, Equal(apisv1alpha1.APIBindingPhaseBound)))

	// And check the APIs are imported into the test workspace
	test.Expect(HasImportedAPIs(test, workspace, kuadrantv1.SchemeGroupVersion.WithKind("DNSRecord"))(test)).
		Should(BeTrue())

	// Create a namespace
	namespace := test.NewTestNamespace(InWorkspace(workspace))

	name := "echo"

	// Create the Deployment
	_, err := test.Client().Core().Cluster(logicalcluster.From(namespace)).AppsV1().Deployments(namespace.Name).
		Apply(test.Ctx(), DeploymentConfiguration(namespace.Name, name), ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())
	defer func() {
		test.Expect(test.Client().Core().Cluster(logicalcluster.From(namespace)).AppsV1().Deployments(namespace.Name).
			Delete(test.Ctx(), name, metav1.DeleteOptions{})).
			To(Succeed())
	}()

	// Create the Service
	_, err = test.Client().Core().Cluster(logicalcluster.From(namespace)).CoreV1().Services(namespace.Name).
		Apply(test.Ctx(), ServiceConfiguration(namespace.Name, name, map[string]string{}), ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())
	defer func() {
		test.Expect(test.Client().Core().Cluster(logicalcluster.From(namespace)).CoreV1().Services(namespace.Name).
			Delete(test.Ctx(), name, metav1.DeleteOptions{})).
			To(Succeed())
	}()

	// Create the Ingress
	_, err = test.Client().Core().Cluster(logicalcluster.From(namespace)).NetworkingV1().Ingresses(namespace.Name).
		Apply(test.Ctx(), IngressConfiguration(namespace.Name, name, "test.gblb.com"), ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())
	defer func() {
		test.Expect(test.Client().Core().Cluster(logicalcluster.From(namespace)).NetworkingV1().Ingresses(namespace.Name).
			Delete(test.Ctx(), name, metav1.DeleteOptions{})).
			To(Succeed())
	}()

	// Wait until the Ingress is reconciled with the load balancer Ingresses
	test.Eventually(Ingress(test, namespace, name)).WithTimeout(TestTimeoutMedium).Should(And(
		WithTransform(Annotations, And(
			HaveKey(access.ANNOTATION_HCG_HOST),
			HaveKey(access.ANNOTATION_PENDING_CUSTOM_HOSTS),
		)),
		WithTransform(Labels, And(
			HaveKey(access.LABEL_HAS_PENDING_HOSTS),
		)),
		WithTransform(LoadBalancerIngresses, HaveLen(1)),
		Satisfy(HostsEqualsToGeneratedHost),
	))

	// Retrieve the Ingress
	ingress := GetIngress(test, namespace, name)
	accessor := &access.IngressAccessor{Ingress: ingress}
	hostname := accessor.GetAnnotations()[access.ANNOTATION_HCG_HOST]
	secretName := reconcilers.TLSSecretName(accessor)

	// Check the Ingress TLS spec
	test.Expect(ingress).To(WithTransform(IngressTLS, ConsistOf(
		networkingv1.IngressTLS{
			Hosts:      []string{hostname},
			SecretName: secretName,
		}),
	))

	// Check the TLS Secret
	issuer := env.GetEnvString("GLBC_TLS_PROVIDER", "glbc-ca")
	fields := map[string]types.GomegaMatcher{
		"DNSNames": ConsistOf(hostname),
	}
	switch issuer {

	case "glbc-ca":
		fields["Issuer"] = Equal(pkix.Name{
			Organization: []string{"Kuadrant"},
			Names: []pkix.AttributeTypeAndValue{
				{
					Type:  []int{2, 5, 4, 10},
					Value: "Kuadrant",
				},
			},
		})

	case "le-staging":
		fields["Issuer"] = Equal(pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"(STAGING) Let's Encrypt"},
			CommonName:   "(STAGING) Artificial Apricot R3",
			Names: []pkix.AttributeTypeAndValue{
				{
					Type:  []int{2, 5, 4, 6},
					Value: "US",
				},
				{
					Type:  []int{2, 5, 4, 10},
					Value: "(STAGING) Let's Encrypt",
				},
				{
					Type:  []int{2, 5, 4, 3},
					Value: "(STAGING) Artificial Apricot R3",
				},
			},
		})
		fields["Subject"] = Equal(pkix.Name{
			CommonName: hostname,
			Names: []pkix.AttributeTypeAndValue{
				{
					Type:  []int{2, 5, 4, 3},
					Value: hostname,
				},
			},
		})

	case "le-production":
		fields["Issuer"] = Equal(pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Let's Encrypt"},
			CommonName:   "Artificial Apricot R3",
			Names: []pkix.AttributeTypeAndValue{
				{
					Type:  []int{2, 5, 4, 6},
					Value: "US",
				},
				{
					Type:  []int{2, 5, 4, 10},
					Value: "Let's Encrypt",
				},
				{
					Type:  []int{2, 5, 4, 3},
					Value: "R3",
				},
			},
		})
		fields["Subject"] = Equal(pkix.Name{
			CommonName: hostname,
			Names: []pkix.AttributeTypeAndValue{
				{
					Type:  []int{2, 5, 4, 3},
					Value: hostname,
				},
			},
		})
	}

	test.Eventually(Secret(test, namespace, secretName)).
		WithTimeout(TestTimeoutMedium).
		Should(WithTransform(Certificate, PointTo(MatchFields(IgnoreExtras, fields))))
}
