package v1

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var podsGated = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "may",
	Subsystem: "pods",
	Name:      "gated",
})

func init() {
	metrics.Registry.MustRegister(podsGated)
}
