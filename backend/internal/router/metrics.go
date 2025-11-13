package router

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	QuotesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lucendex_quotes_total",
			Help: "Total quote requests",
		},
		[]string{"outcome"},
	)

	QuoteLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lucendex_quote_latency_ms",
			Help:    "Quote generation latency",
			Buckets: []float64{10, 25, 50, 100, 200, 500, 1000},
		},
		[]string{"outcome"},
	)

	CircuitBreakerMetric = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lucendex_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"pair"},
	)

	CacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lucendex_cache_hits_total",
			Help: "KV cache hits",
		},
	)

	CacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lucendex_cache_misses_total",
			Help: "KV cache misses",
		},
	)

	MemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lucendex_kv_memory_bytes",
			Help: "KV store memory usage",
		},
	)
)
