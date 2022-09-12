package reconcilers

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/kuadrant/kcp-glbc/pkg/access"
	"github.com/kuadrant/kcp-glbc/pkg/metrics"
	"github.com/kuadrant/kcp-glbc/pkg/tls"
)

const (
	issuerLabel          = "issuer"
	resultLabel          = "result"
	resultLabelSucceeded = "succeeded"
	resultLabelFailed    = "failed"
)

type Reconciler interface {
	Reconcile(ctx context.Context, accessor access.Accessor) (access.ReconcileStatus, error)
}

var (
	IngressObjectTimeToAdmission = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "glbc_ingress_managed_object_time_to_admission",
			Help: "Duration of the ingress object admission",
			Buckets: []float64{
				1 * time.Second.Seconds(),
				5 * time.Second.Seconds(),
				10 * time.Second.Seconds(),
				15 * time.Second.Seconds(),
				30 * time.Second.Seconds(),
				45 * time.Second.Seconds(),
				1 * time.Minute.Seconds(),
				2 * time.Minute.Seconds(),
				5 * time.Minute.Seconds(),
			},
		})

	TlsCertificateRequestCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glbc_tls_certificate_pending_request_count",
			Help: "GLBC TLS certificate pending request count",
		},
		[]string{
			issuerLabel,
		},
	)

	// TlsCertificateRequestTotal is a prometheus counter metrics which holds the total
	// number of TLS certificate requests.
	TlsCertificateRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "glbc_tls_certificate_request_total",
			Help: "GLBC TLS certificate total number of requests",
		},
		[]string{
			issuerLabel,
			resultLabel,
		},
	)

	// TlsCertificateIssuanceDuration is a prometheus metric which records the duration
	// of TLS certificate issuance.
	TlsCertificateIssuanceDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "glbc_tls_certificate_issuance_duration_seconds",
			Help: "GLBC TLS certificate issuance duration",
			Buckets: []float64{
				1 * time.Second.Seconds(),
				5 * time.Second.Seconds(),
				10 * time.Second.Seconds(),
				15 * time.Second.Seconds(),
				30 * time.Second.Seconds(),
				45 * time.Second.Seconds(),
				1 * time.Minute.Seconds(),
				2 * time.Minute.Seconds(),
				5 * time.Minute.Seconds(),
			},
		},
		[]string{
			issuerLabel,
			resultLabel,
		},
	)

	// TlsCertificateRequestErrors is a prometheus counter metrics which holds the total
	// number of failed TLS certificate requests.
	TlsCertificateRequestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "glbc_tls_certificate_request_errors_total",
			Help: "GLBC TLS certificate total number of request errors",
		},
		// TODO: check if it's possible to add an error/code label
		[]string{
			issuerLabel,
		},
	)
)

func init() {
	// Register metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		IngressObjectTimeToAdmission,
		TlsCertificateRequestCount,
		TlsCertificateRequestErrors,
		TlsCertificateRequestTotal,
		TlsCertificateIssuanceDuration,
	)
}

func InitMetrics(provider tls.Provider) {
	issuer := provider.IssuerID()
	TlsCertificateRequestCount.WithLabelValues(issuer).Set(0)
	TlsCertificateRequestTotal.WithLabelValues(issuer, resultLabelSucceeded).Add(0)
	TlsCertificateRequestTotal.WithLabelValues(issuer, resultLabelFailed).Add(0)
	TlsCertificateRequestErrors.WithLabelValues(issuer).Add(0)
}
