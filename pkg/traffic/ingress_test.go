package traffic

import (
	"encoding/json"
	"fmt"
	"testing"

	workload "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplyTransforms(t *testing.T) {
	cases := []struct {
		Name string
		// OriginalIngress is the ingress as the user created it
		OriginalIngress *networkingv1.Ingress
		// ReconciledIngress is the ingress after the controller has done its work and ready to save it
		ReconciledIngress *networkingv1.Ingress
		ExpectErr         bool
	}{
		{
			Name: "test original spec not changed post reconcile and transforms applied single host",
			OriginalIngress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"experimental.status.workload.kcp.dev/c1": "",
					},
					Labels: map[string]string{
						"state.workload.kcp.dev/c1": "Sync",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "test.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "test"}}},
									},
								},
							},
						},
					},
				},
			},
			ReconciledIngress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"experimental.status.workload.kcp.dev/c1": "",
						"experimental.status.workload.kcp.dev/c2": "",
					},
					Labels: map[string]string{
						"state.workload.kcp.dev/c1": "Sync",
						"state.workload.kcp.dev/c2": "Sync",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "guid.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "test"}}},
									},
								},
							},
						},
					},
					TLS: []networkingv1.IngressTLS{
						{Hosts: []string{"guid.example.com"}, SecretName: "test"},
					},
				},
			},
		},
		{
			Name: "test original spec not changed post reconcile and transforms applied multiple verified hosts",
			OriginalIngress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"experimental.status.workload.kcp.dev/c1": "",
					},
					Labels: map[string]string{
						"state.workload.kcp.dev/c1": "Sync",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "test.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "test"}}},
									},
								},
							},
						},
						{
							Host: "test2.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "test"}}},
									},
								},
							},
						},
					},
				},
			},
			ReconciledIngress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"experimental.status.workload.kcp.dev/c1": "",
						"experimental.status.workload.kcp.dev/c2": "",
					},
					Labels: map[string]string{
						"state.workload.kcp.dev/c1": "Sync",
						"state.workload.kcp.dev/c2": "Sync",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "test.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "test"}}},
									},
								},
							},
						},
						{
							Host: "api.test.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "test"}}},
									},
								},
							},
						},
						{
							Host: "guid.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "test"}}},
									},
								},
							},
						},
					},
					TLS: []networkingv1.IngressTLS{
						{Hosts: []string{"guid.example.com"}, SecretName: "glbc"},
						{Hosts: []string{"test.com", "api.test.com"}, SecretName: "test"},
					},
				},
			},
		},
	}

	var validateTransform = func(expectedTransform networkingv1.IngressSpec, transformed *Ingress) error {
		st := transformed.GetSyncTargets()
		for _, target := range st {
			// ensure each target has a transform value set and it is correct
			if _, ok := transformed.Annotations[workload.ClusterSpecDiffAnnotationPrefix+target]; !ok {
				return fmt.Errorf("expected a transformation for sync target " + target)
			}
			transforms := transformed.Annotations[workload.ClusterSpecDiffAnnotationPrefix+target]
			patches := []struct {
				Path  string                   `json:"path"`
				Op    string                   `json:"op"`
				Value []map[string]interface{} `json:"value"`
			}{}
			if err := json.Unmarshal([]byte(transforms), &patches); err != nil {
				return fmt.Errorf("failed to unmarshal patch %s", err)
			}
			//ensure there is a rules and tls patch and they have the correct value
			rulesPatch := false
			tlsPatch := false
			for _, p := range patches {
				if p.Path == "/rules" {
					rulesPatch = true
					rules := []networkingv1.IngressRule{}
					b, err := json.Marshal(p.Value)
					if err != nil {
						return fmt.Errorf("failed to marshal rules %s", err)
					}
					json.Unmarshal(b, &rules)
					if !equality.Semantic.DeepEqual(rules, expectedTransform.Rules) {
						return fmt.Errorf("expected the rules in the transform to match the rules in transformed ingress")
					}
				}
				if p.Path == "/tls" {
					tlsPatch = true
					tls := []networkingv1.IngressTLS{}
					b, err := json.Marshal(p.Value)
					if err != nil {
						return fmt.Errorf("failed to marshal rules %s", err)
					}
					json.Unmarshal(b, &tls)
					if !equality.Semantic.DeepEqual(tls, expectedTransform.TLS) {
						return fmt.Errorf("expected the tls section in the transform to match the tls in transformed ingress")
					}
				}
			}
			if !rulesPatch {
				return fmt.Errorf("expected to find a rules patch but one was missing")
			}
			if !tlsPatch {
				return fmt.Errorf("expected to find a tls patch but one was missing")
			}

		}
		return nil
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			// take a copy before we apply transforms
			transformedCopy := testCase.ReconciledIngress.DeepCopy()
			reconciled := NewIngress(testCase.ReconciledIngress)
			original := NewIngress(testCase.OriginalIngress)
			err := reconciled.Transform(original)
			// after the transform is done, we should have the specs of the original and transformed remain the same
			if !equality.Semantic.DeepEqual(testCase.OriginalIngress.Spec, testCase.ReconciledIngress.Spec) {
				t.Fatalf("expected the spec of the orignal and transformed to have remained the same. Expected %v Got %v", testCase.OriginalIngress.Spec, testCase.ReconciledIngress.Spec)
			}
			// we should now have annotations applying the transforms
			if err := validateTransform(transformedCopy.Spec, reconciled); err != nil {
				t.Fatalf("transforms were invalid %s", err)
			}
			if testCase.ExpectErr {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("did not expect an error but got %v", err)
				}
			}
		})
	}

}

func TestGetDNSTargets(t *testing.T) {

}
