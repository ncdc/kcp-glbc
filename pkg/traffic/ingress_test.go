package traffic_test

import (
	"context"
	"testing"

	kuadrantv1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/traffic"
	testSupport "github.com/kuadrant/kcp-glbc/test/support"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func defaultTestIngress(hosts []string, backend string, tls []networkingv1.IngressTLS) *networkingv1.Ingress {

	rules := []networkingv1.IngressRule{}
	for _, h := range hosts {
		rules = append(rules, networkingv1.IngressRule{
			Host: h,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: backend}}},
					},
				},
			},
		})
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: networkingv1.IngressSpec{
			Rules: rules,
		},
	}

	ing.Spec.TLS = tls

	return ing
}

func TestProcessCustomHosts(t *testing.T) {
	cases := []struct {
		Name                string
		OriginalIngress     func() *networkingv1.Ingress
		ExpectedIngress     func() *networkingv1.Ingress
		DomainVerifications *kuadrantv1.DomainVerificationList
		ExpectErr           bool
	}{
		{
			Name: "test unverified host removed and replaced with glbc host",
			OriginalIngress: func() *networkingv1.Ingress {
				ing := defaultTestIngress([]string{"example.com"}, "test",
					[]networkingv1.IngressTLS{
						{Hosts: []string{"example.com"}, SecretName: "test"},
						{Hosts: []string{"guid.hcg.com"}, SecretName: "guid-hcg-com"},
					})
				ing.Annotations = map[string]string{traffic.ANNOTATION_HCG_HOST: "guid.hcg.com"}
				return ing
			},
			ExpectedIngress: func() *networkingv1.Ingress {
				ing := defaultTestIngress([]string{"guid.hcg.com"}, "test", []networkingv1.IngressTLS{
					{Hosts: []string{"guid.hcg.com"}, SecretName: "guid-hcg-com"},
				})
				ing.Annotations = map[string]string{traffic.ANNOTATION_HCG_HOST: "guid.hcg.com", traffic.ANNOTATION_HCG_CUSTOM_HOST_REPLACED: "[example.com]"}
				ing.Labels = map[string]string{traffic.LABEL_HAS_PENDING_HOSTS: "true"}
				return ing
			},
			DomainVerifications: &kuadrantv1.DomainVerificationList{},
		},
		{
			Name: "test verfied host left in place and glbc rule appended",
			OriginalIngress: func() *networkingv1.Ingress {
				ing := defaultTestIngress([]string{"example.com"}, "test",
					[]networkingv1.IngressTLS{
						{Hosts: []string{"example.com"}, SecretName: "test"},
						{Hosts: []string{"guid.hcg.com"}, SecretName: "guid-hcg-com"},
					})
				ing.Annotations = map[string]string{traffic.ANNOTATION_HCG_HOST: "guid.hcg.com"}
				return ing
			},
			ExpectedIngress: func() *networkingv1.Ingress {
				ing := defaultTestIngress([]string{"example.com", "guid.hcg.com"}, "test",
					[]networkingv1.IngressTLS{
						{Hosts: []string{"example.com"}, SecretName: "test"},
						{Hosts: []string{"guid.hcg.com"}, SecretName: "guid-hcg-com"},
					})
				ing.Annotations = map[string]string{traffic.ANNOTATION_HCG_HOST: "guid.hcg.com"}
				return ing
			},
			DomainVerifications: &kuadrantv1.DomainVerificationList{
				Items: []kuadrantv1.DomainVerification{{
					Spec: kuadrantv1.DomainVerificationSpec{
						Domain: "example.com",
					},
					Status: kuadrantv1.DomainVerificationStatus{
						Verified: true,
					},
				}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ing := traffic.NewIngress(tc.OriginalIngress())
			err := ing.ProcessCustomHosts(context.TODO(), tc.DomainVerifications, nil, nil)
			if tc.ExpectErr && err == nil {
				t.Fatalf("expected an error for ProcessCustomHosts but got none")
			}
			if !tc.ExpectErr && err != nil {
				t.Fatalf("did not expect an error for ProcessCustomHosts but got %s ", err)
			}
			expected := traffic.NewIngress(tc.ExpectedIngress())
			if !equality.Semantic.DeepEqual(ing.Spec, expected.Spec) {
				t.Log("exp spec", expected.Spec)
				t.Log("got spec", ing.Spec)
				t.Fatalf("expected processi ingress to match the expected ingress ")
			}
			if !equality.Semantic.DeepEqual(ing.Annotations, expected.Annotations) {
				t.Log("exp annotations", expected.Annotations)
				t.Log("got annotations", ing.Annotations)
			}
			if !equality.Semantic.DeepEqual(ing.Labels, expected.Labels) {
				t.Log("exp labels", expected.Labels)
				t.Log("got labels", ing.Labels)
			}
		})
	}
}

func TestApplyTransformsIngress(t *testing.T) {
	cases := []struct {
		Name string
		// OriginalIngress is the ingress as the user created it
		OriginalIngress func() *networkingv1.Ingress
		// ReconciledIngress is the ingress after the controller has done its work and ready to save it
		ReconciledIngress func() *networkingv1.Ingress
		ExpectTLSDiff     bool
		ExpectRulesDiff   bool
		ExpectErr         bool
	}{
		{
			Name: "test original spec not changed post reconcile and transforms applied single host",
			OriginalIngress: func() *networkingv1.Ingress {
				return defaultTestIngress([]string{"test.com"}, "test", nil)
			},
			ReconciledIngress: func() *networkingv1.Ingress {
				ing := defaultTestIngress([]string{"guid.example.com"}, "test", []networkingv1.IngressTLS{
					{Hosts: []string{"guid.example.com"}, SecretName: "glbc"},
				})
				ing.Labels = map[string]string{
					"state.workload.kcp.dev/c1": "Sync",
				}
				return ing
			},
			ExpectTLSDiff:   true,
			ExpectRulesDiff: true,
		},
		{
			Name: "test original spec not changed post reconcile and transforms applied multiple verified hosts",
			OriginalIngress: func() *networkingv1.Ingress {
				return defaultTestIngress([]string{"test.com", "api.test.com"}, "test", []networkingv1.IngressTLS{
					{Hosts: []string{"test.com", "api.test.com"}, SecretName: "test"}})
			},
			ReconciledIngress: func() *networkingv1.Ingress {
				ing := defaultTestIngress([]string{"test.com", "api.test.com", "guid.example.com"}, "test", []networkingv1.IngressTLS{
					{Hosts: []string{"guid.example.com"}, SecretName: "glbc"},
					{Hosts: []string{"test.com", "api.test.com"}, SecretName: "test"},
				})
				ing.Labels = map[string]string{
					"state.workload.kcp.dev/c1": "Sync",
				}
				return ing
			},
			ExpectTLSDiff:   true,
			ExpectRulesDiff: true,
		},
		{
			Name: "test original spec not changed post reconcile if generated host in spec",
			OriginalIngress: func() *networkingv1.Ingress {
				return defaultTestIngress([]string{"guid.example.com"}, "test", []networkingv1.IngressTLS{
					{Hosts: []string{"test.com"}, SecretName: "test"}, {Hosts: []string{"guid.example.com"}, SecretName: "glbc"}})
			},
			ReconciledIngress: func() *networkingv1.Ingress {
				ing := defaultTestIngress([]string{"guid.example.com"}, "test", []networkingv1.IngressTLS{
					{Hosts: []string{"guid.example.com"}, SecretName: "glbc"},
				})
				ing.Labels = map[string]string{
					"state.workload.kcp.dev/c1": "Sync",
				}
				return ing
			},
			ExpectTLSDiff: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			// take a copy before we apply transforms this will have all the expected changes to the spec
			reconciled := testCase.ReconciledIngress()
			transformedCopy := reconciled.DeepCopy()
			reconciledIng := traffic.NewIngress(reconciled)
			orignal := testCase.OriginalIngress()
			originalIng := traffic.NewIngress(orignal)
			// Apply transforms, this will reset the spec to the original once done
			err := reconciledIng.Transform(originalIng)
			if testCase.ExpectErr {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("did not expect an error but got %v", err)
				}
			}
			// after the transform is done, we should have the specs of the original and transformed remain the same
			if !equality.Semantic.DeepEqual(orignal.Spec, reconciled.Spec) {
				t.Fatalf("expected the spec of the orignal and transformed to have remained the same. Expected %v Got %v", orignal.Spec, reconciled.Spec)
			}
			t.Log(reconciledIng.Annotations)
			// we should now have annotations applying the transforms. Validate the transformed spec matches the transform annotations.
			if err := testSupport.ValidateTransformedIngress(transformedCopy.Spec, reconciledIng, testCase.ExpectRulesDiff, testCase.ExpectTLSDiff); err != nil {
				t.Fatalf("transforms were invalid %s", err)
			}

		})
	}

}
