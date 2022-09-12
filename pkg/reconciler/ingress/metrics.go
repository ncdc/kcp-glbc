package ingress

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/kuadrant/kcp-glbc/pkg/metrics"
	"github.com/kuadrant/kcp-glbc/pkg/tls"
)

const (
	issuerLabel = "issuer"
)

var (
	ingressObjectTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "glbc_ingress_managed_object_total",
			Help: "Total number of managed ingress object",
		})
	// tlsCertificateSecretCount is a prometheus metric which holds the number of
	// TLS certificates currently managed.
	tlsCertificateSecretCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glbc_tls_certificate_secret_count",
			Help: "GLBC TLS certificate secret count",
		},
		[]string{
			issuerLabel,
		},
	)
)

func init() {
	// Register metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		ingressObjectTotal,
		tlsCertificateSecretCount,
	)
}

func InitMetrics(provider tls.Provider) {
	issuer := provider.IssuerID()
	tlsCertificateSecretCount.WithLabelValues(issuer).Set(0)
}
