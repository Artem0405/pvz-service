package mmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// !!! Начинаются с Большой Буквы !!!
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pvz_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pvz_http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	PVZCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pvz_created_total",
			Help: "Total number of created PVZ.",
		},
	)

	ReceptionsInitiatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pvz_receptions_initiated_total",
			Help: "Total number of initiated receptions.",
		},
	)

	ProductsAddedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pvz_products_added_total",
			Help: "Total number of added products.",
		},
	)
)
