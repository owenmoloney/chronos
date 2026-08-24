package observe

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler returns the prometheus HTTP handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// Global application metrics
var (
	// CronEnqueued tracks jobs successfully made runnable from a cron fire
	CronEnqueued = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chronos_cron_enqueued_total",
		Help: "Jobs successfully made runnable from a cron fire.",
	})

	// JobsClaimed tracks jobs claimed by workers
	JobsClaimed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chronos_jobs_claimed_total",
		Help: "The total number of jobs claimed by workers.",
	})

	// JobsCompleted tracks successfully finished jobs
	JobsCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chronos_jobs_completed_total",
		Help: "The total number of jobs successfully executed.",
	})

	// JobsFailed tracks jobs that dropped out with errors
	JobsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chronos_jobs_failed_total",
		Help: "The total number of jobs that failed execution.",
	})

	// Leader indicates cluster node lease ownership
	Leader = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "chronos_leader",
		Help: "1 if this process holds the lease, else 0.",
	})
)
