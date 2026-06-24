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

	// --- Proxy (Phase 1 data plane) ---

	ProxyConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "nance_proxy_connections_active",
		Help: "Active client TCP connections to the proxy",
	})

	ProxyCommands = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_proxy_commands_total",
		Help: "Commands handled by the proxy",
	}, []string{"tenant", "command"})

	ProxyCommandDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nance_proxy_command_duration_seconds",
		Help:    "Command handling latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"command"})

	ProxyAuthSuccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_proxy_auth_success_total",
		Help: "Successful PLAIN authentications",
	}, []string{"tenant"})

	ProxyAuthFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nance_proxy_auth_failures_total",
		Help: "Failed authentication attempts",
	})

	ProxyBackendErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_proxy_backend_errors_total",
		Help: "Errors talking to tenant backend MongoDB",
	}, []string{"tenant"})

	// --- Cache (Phase 2) ---

	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_cache_hits_total",
		Help: "Cache hits served from Redis",
	}, []string{"tenant", "ns", "command"})

	CacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_cache_misses_total",
		Help: "Cache misses that executed against backend",
	}, []string{"tenant", "ns", "command"})

	CacheBypass = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_cache_bypass_total",
		Help: "Commands that skipped cache",
	}, []string{"tenant", "reason"})

	CacheInvalidations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nance_cache_invalidations_total",
		Help: "Namespace invalidations",
	}, []string{"tenant", "ns", "reason"})

	CacheUnavailable = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nance_cache_redis_unavailable_total",
		Help: "Redis errors on hot path (fail-open)",
	})

	CacheResultBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nance_cache_result_bytes",
		Help:    "Size of payloads stored in cache",
		Buckets: []float64{256, 1024, 4096, 16384, 65536, 262144, 1048576},
	}, []string{"tenant"})

	CacheLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nance_cache_latency_seconds",
		Help:    "Cache path latency (hit vs miss populate)",
		Buckets: prometheus.DefBuckets,
	}, []string{"path"}) // hit | miss
)
