package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TenantCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nance_tenants_created_total",
		Help: "Number of tenants created",
	})

	BackendTestSuccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_backend_test_success_total",
		Help: "Successful backend connection tests",
	}, []string{"tenant"})

	PolicyUpdates = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_policy_updates_total",
		Help: "Cache policy updates",
	}, []string{"tenant", "type"})
)
