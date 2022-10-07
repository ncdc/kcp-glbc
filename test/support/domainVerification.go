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
	"github.com/kcp-dev/logicalcluster/v2"
	"github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kuadrantv1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
)

func DomainVerification(t Test, clusterName logicalcluster.Name, name string) func(g gomega.Gomega) *kuadrantv1.DomainVerification {
	return func(g gomega.Gomega) *kuadrantv1.DomainVerification {
		domainVerification, err := t.Client().Kuadrant().Cluster(clusterName).KuadrantV1().DomainVerifications().Get(t.Ctx(), name, metav1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return domainVerification
	}
}

func DomainVerificationFor(verification *kuadrantv1.DomainVerification) string {
	return verification.Spec.Domain
}

func DomainVerified(verification *kuadrantv1.DomainVerification) bool {
	return verification.Status.Verified
}

func DomainToken(verification *kuadrantv1.DomainVerification) string {
	return verification.Status.Token
}
