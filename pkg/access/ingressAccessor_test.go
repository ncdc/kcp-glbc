package access

import (
	"encoding/json"
	"fmt"
	"testing"

	workload "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//TODO add unit test for transforms

func TestApplyTransforms(t *testing.T) {
	cases := []struct {
		Name               string
		OriginalIngress    *networkingv1.Ingress
		TransformedIngress *networkingv1.Ingress
		ExpectErr          bool
	}{
		{
			Name: "test original spec not changed and transforms applied",
			OriginalIngress: &networkingv1.Ingress{
				ObjectMeta: v1.ObjectMeta{
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
			TransformedIngress: &networkingv1.Ingress{
				ObjectMeta: v1.ObjectMeta{
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
	}

	var validateTransform = func(expectedTransform networkingv1.IngressSpec, transformed *IngressAccessor) error {
		st := transformed.GetSyncTargets()
		for _, target := range st {
			// ensure each target has a tranform value set and it is correct
			if _, ok := transformed.Annotations[workload.ClusterSpecDiffAnnotationPrefix+target]; !ok {
				return fmt.Errorf("expected a transformation for sync target " + target)
			}
			transforms := transformed.Annotations[workload.ClusterSpecDiffAnnotationPrefix+target]
			fmt.Println(transforms)
			patches := []struct {
				Path  string `json:"path"`
				Op    string `json:"op"`
				Value string `json:"value"`
			}{}
			json.Unmarshal([]byte(transforms), &patches)
			//ensure there is a rules and tls patch
			fmt.Println(patches)
			rulesPatch := false
			tlsPatch := false
			for _, p := range patches {
				if p.Path == "/rules" {
					rulesPatch = true
					rules := []networkingv1.IngressRule{}
					json.Unmarshal([]byte(p.Value), &rules)
					fmt.Println(rules)
					if !equality.Semantic.DeepEqual(rules, expectedTransform.Rules) {
						return fmt.Errorf("expected the rules in the transform to match the rules in transformed ingress")
					}
				}
				if p.Path == "/tls" {
					tlsPatch = true
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
			transformedCopy := testCase.TransformedIngress.DeepCopy()
			transformed := NewIngressAccessor(testCase.TransformedIngress)
			original := NewIngressAccessor(testCase.OriginalIngress)
			err := transformed.ApplyTransforms(original)
			// after the transform is done, we should have the specs of the original and transformed remain the same
			if !equality.Semantic.DeepEqual(testCase.OriginalIngress.Spec, testCase.TransformedIngress.Spec) {
				t.Fatalf("expected the spec of the orignal and transformed to have remained the same. Expected %v Got %v", testCase.OriginalIngress.Spec, testCase.TransformedIngress.Spec)
			}
			// we should now have annotations applying the transforms
			if err := validateTransform(transformedCopy.Spec, transformed); err != nil {
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

func TestGetTargets(t *testing.T) {

}
