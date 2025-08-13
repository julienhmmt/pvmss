package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	initOnce sync.Once

	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Proxmox client metrics
	proxmoxRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxmox_requests_total",
			Help: "Total number of Proxmox API requests.",
		},
		[]string{"method", "endpoint", "status", "outcome"},
	)

	proxmoxRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxmox_request_duration_seconds",
			Help:    "Proxmox API request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

// Init registers all collectors exactly once.
func Init() {
	initOnce.Do(func() {
		prometheus.MustRegister(httpRequestsTotal)
		prometheus.MustRegister(httpRequestDuration)
		prometheus.MustRegister(proxmoxRequestsTotal)
		prometheus.MustRegister(proxmoxRequestDuration)
	})
}

// Handler exposes the /metrics HTTP handler.
func Handler() http.Handler {
	Init()
	return promhttp.Handler()
}

// HTTPMetricsMiddleware captures basic HTTP metrics.
type statusCapturingWriter struct {
	w      http.ResponseWriter
	status int
}

func (s *statusCapturingWriter) Header() http.Header         { return s.w.Header() }
func (s *statusCapturingWriter) Write(b []byte) (int, error) { return s.w.Write(b) }
func (s *statusCapturingWriter) WriteHeader(code int) {
	s.status = code
	s.w.WriteHeader(code)
}

func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	Init()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		scw := &statusCapturingWriter{w: w, status: 200}
		next.ServeHTTP(scw, r)

		path := r.URL.Path
		method := r.Method
		status := scw.status
		httpRequestsTotal.WithLabelValues(method, path, itoa(status)).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	})
}

// ObserveProxmox records Proxmox client request metrics.
func ObserveProxmox(method, endpoint string, status int, outcome string, start time.Time) {
	Init()
	proxmoxRequestsTotal.WithLabelValues(method, endpoint, itoa(status), outcome).Inc()
	proxmoxRequestDuration.WithLabelValues(method, endpoint).Observe(time.Since(start).Seconds())
}

// Small helper to avoid importing strconv everywhere.
func itoa(i int) string {
	// tiny fastpath for common statuses
	switch i {
	case 200:
		return "200"
	case 201:
		return "201"
	case 202:
		return "202"
	case 204:
		return "204"
	case 400:
		return "400"
	case 401:
		return "401"
	case 403:
		return "403"
	case 404:
		return "404"
	case 409:
		return "409"
	case 429:
		return "429"
	case 500:
		return "500"
	case 502:
		return "502"
	case 503:
		return "503"
	case 504:
		return "504"
	default:
		// fallback
		return strconv.Itoa(i)
	}
}
