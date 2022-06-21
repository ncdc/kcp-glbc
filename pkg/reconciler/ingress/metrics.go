package ingress

import (
	"time"

	"github.com/kuadrant/kcp-glbc/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ingressObjectTimeToAdmission = prometheus.NewHistogram(
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

	ingressObjectTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "glbc_ingress_managed_object_total",
			Help: "Total number of managed ingress object",
		})
)

func init() {
	// Register metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		ingressObjectTimeToAdmission,
		ingressObjectTotal,
	)
}
