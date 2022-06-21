// This file is originally based on https://github.com/kubernetes-sigs/controller-runtime/blob/f46919744bee01060c9084a285e049afffd38c9d/pkg/metrics/client_go_adapter.go
// which uses the same license as this project (Apache License Version 2.0)
// It is stripped down to provide request result & latency metrics for rest client usage.
// There used to be reflector metrics in client-go, however they were removed due to a
// memory leak: https://github.com/kubernetes/kubernetes/pull/74636
// That PR also has some discussion about the usefulness of reflector metrics.
// If any additional metrics (existing or new) in client-go need to be exposed, they can
// be registered and a provider set in this file

package metrics

import (
	"context"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	clientmetrics "k8s.io/client-go/tools/metrics"
)

// this file contains setup logic to initialize the myriad of places
// that client-go registers metrics.  We copy the names and formats
// from Kubernetes so that we match the core controllers.

// Metrics subsystem and all of the keys used by the rest client.
const (
	RestClientSubsystem = "rest_client"
	LatencyKey          = "request_latency_seconds"
	ResultKey           = "requests_total"
)

var (
	// client metrics.

	// To prevent cardinality explosion, only the verb is added as a label to latency metrics
	RequestLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: RestClientSubsystem,
		Name:      LatencyKey,
		Help:      "Request latency in seconds. Broken down by verb.",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
	}, []string{"verb"})

	requestResult = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: RestClientSubsystem,
		Name:      ResultKey,
		Help:      "Number of HTTP requests, partitioned by status code, method, and host.",
	}, []string{"code", "method", "host"})
)

func init() {
	registerClientMetrics()
}

// registerClientMetrics sets up the client latency metrics from client-go.
func registerClientMetrics() {
	// register the metrics with our registry
	Registry.MustRegister(requestResult)
	Registry.MustRegister(RequestLatency)

	// register the metrics with client-go
	clientmetrics.Register(clientmetrics.RegisterOpts{
		RequestResult:  &resultAdapter{metric: requestResult},
		RequestLatency: &LatencyAdapter{metric: RequestLatency},
	})
}

// this section contains adapters, implementations, and other sundry organic, artisanally
// hand-crafted syntax trees required to convince client-go that it actually wants to let
// someone use its metrics.

// Client metrics adapters (method #1 for client-go metrics),
// copied (more-or-less directly) from k8s.io/kubernetes setup code
// (which isn't anywhere in an easily-importable place).

// LatencyAdapter implements LatencyMetric.
type LatencyAdapter struct {
	metric *prometheus.HistogramVec
}

// Observe increments the request latency metric for the given verb/URL.
func (l *LatencyAdapter) Observe(_ context.Context, verb string, u url.URL, latency time.Duration) {
	l.metric.WithLabelValues(verb).Observe(latency.Seconds())
}

type resultAdapter struct {
	metric *prometheus.CounterVec
}

func (r *resultAdapter) Increment(_ context.Context, code, method, host string) {
	r.metric.WithLabelValues(code, method, host).Inc()
}
