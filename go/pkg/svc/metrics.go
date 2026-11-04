package svc

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"udyam/pkg/kafkax"
)

// metrics is a tiny Prometheus text-format registry: request counts and latency per route and
// status, Kafka consumer counters, and gauges a service registers (e.g. outbox backlog). No client
// library: the format is plain text and the numbers are few.
type metrics struct {
	mu       sync.Mutex
	requests map[[3]string]uint64  // method, route, status class
	latency  map[[2]string]float64 // method, route -> seconds sum
	count    map[[2]string]uint64
	gauges   map[string]func() float64
	start    time.Time
}

func newMetrics() *metrics {
	return &metrics{requests: map[[3]string]uint64{}, latency: map[[2]string]float64{}, count: map[[2]string]uint64{},
		gauges: map[string]func() float64{}, start: time.Now()}
}

// route collapses IDs so label cardinality stays bounded: /orders/9f2.../timeline -> /orders/:id/timeline.
func route(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if i > 0 && (len(p) > 12 || strings.ContainsAny(p, "0123456789")) && p != "v1" {
			parts[i] = ":id"
		}
	}
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return "/" + strings.Join(parts, "/")
}

func (m *metrics) observe(method, path string, status int, d time.Duration) {
	r := route(path)
	class := fmt.Sprintf("%dxx", status/100)
	m.mu.Lock()
	m.requests[[3]string{method, r, class}]++
	m.latency[[2]string{method, r}] += d.Seconds()
	m.count[[2]string{method, r}]++
	m.mu.Unlock()
}

// Gauge registers a value read at scrape time, e.g. a.Gauge("outbox_pending", ...).
func (a *App) Gauge(name string, f func() float64) {
	a.metrics.mu.Lock()
	a.metrics.gauges[name] = f
	a.metrics.mu.Unlock()
}

func (m *metrics) handler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		m.mu.Lock()
		fmt.Fprintf(&b, "# HELP udyam_http_requests_total HTTP requests by route and status class.\n# TYPE udyam_http_requests_total counter\n")
		keys := make([][3]string, 0, len(m.requests))
		for k := range m.requests {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return strings.Join(keys[i][:], " ") < strings.Join(keys[j][:], " ") })
		for _, k := range keys {
			fmt.Fprintf(&b, "udyam_http_requests_total{service=%q,method=%q,route=%q,status=%q} %d\n", service, k[0], k[1], k[2], m.requests[k])
		}
		fmt.Fprintf(&b, "# HELP udyam_http_request_seconds HTTP request latency.\n# TYPE udyam_http_request_seconds summary\n")
		lk := make([][2]string, 0, len(m.count))
		for k := range m.count {
			lk = append(lk, k)
		}
		sort.Slice(lk, func(i, j int) bool { return lk[i][0]+lk[i][1] < lk[j][0]+lk[j][1] })
		for _, k := range lk {
			fmt.Fprintf(&b, "udyam_http_request_seconds_sum{service=%q,method=%q,route=%q} %.6f\n", service, k[0], k[1], m.latency[k])
			fmt.Fprintf(&b, "udyam_http_request_seconds_count{service=%q,method=%q,route=%q} %d\n", service, k[0], k[1], m.count[k])
		}
		gauges := make([]string, 0, len(m.gauges))
		for g := range m.gauges {
			gauges = append(gauges, g)
		}
		sort.Strings(gauges)
		fns := make([]func() float64, len(gauges))
		for i, g := range gauges {
			fns[i] = m.gauges[g]
		}
		m.mu.Unlock()
		processed, retried, dead := kafkax.Counters()
		fmt.Fprintf(&b, "# TYPE udyam_kafka_events_total counter\nudyam_kafka_events_total{service=%q,result=\"processed\"} %d\n", service, processed)
		fmt.Fprintf(&b, "udyam_kafka_events_total{service=%q,result=\"retried\"} %d\nudyam_kafka_events_total{service=%q,result=\"dead_lettered\"} %d\n", service, retried, service, dead)
		for i, g := range gauges {
			fmt.Fprintf(&b, "# TYPE udyam_%s gauge\nudyam_%s{service=%q} %g\n", g, g, service, fns[i]())
		}
		fmt.Fprintf(&b, "# TYPE udyam_uptime_seconds gauge\nudyam_uptime_seconds{service=%q} %.0f\n", service, time.Since(m.start).Seconds())
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(b.String()))
	}
}
