//go:build performance && ingress

package performance

import (
	"fmt"
	"os"
	"sync"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/kuadrant/kcp-glbc/pkg/util/env"
	. "github.com/kuadrant/kcp-glbc/test/smoke"
	. "github.com/kuadrant/kcp-glbc/test/support"
)

func TestIngress(t *testing.T) {
	test := With(t)
	test.T().Parallel()

	// To run this test you must know the DNSZone and Domain the target GLBC instance is configured to use.
	// Check the a DNS zone id has been set
	zoneID := os.Getenv("AWS_DNS_PUBLIC_ZONE_ID")
	test.Expect(zoneID).NotTo(Equal(""))
	// Check the a DNS domain name has been set
	glbcDomain := os.Getenv("GLBC_DOMAIN")
	test.Expect(glbcDomain).NotTo(Equal(""))

	workspaceCount := env.GetEnvInt(TestWorkspaceCount, DefaultTestWorkspaceCount)
	test.Expect(workspaceCount > 0).To(BeTrue())

	ingressCount := env.GetEnvInt(TestIngressCount, DefaultTestIngressCount)
	test.Expect(ingressCount > 0).To(BeTrue())

	test.T().Log(fmt.Sprintf("Creating %d Workspaces, each with %d Ingresses", workspaceCount, ingressCount))

	// Create Workspaces and run ingress test in each
	wg := sync.WaitGroup{}
	for i := 1; i <= workspaceCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			TestIngressBasic(test, ingressCount, zoneID, glbcDomain)
		}()
	}
	wg.Wait()
}
