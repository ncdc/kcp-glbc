//go:build smoke

package smoke

import (
	"os"
	"testing"

	. "github.com/kuadrant/kcp-glbc/test/support"
	. "github.com/onsi/gomega"
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

	TestIngressBasic(test, 1, zoneID, glbcDomain)
}
