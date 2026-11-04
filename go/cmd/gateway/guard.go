package main

import (
	"crypto/subtle"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"udyam/pkg/svc"
)

// ---- rate limiting -----------------------------------------------------------------------------

// bucket is a token bucket: rate tokens per second, up to burst.
type bucket struct {
	tokens float64
	last   time.Time
}

// limiter keeps one bucket per client IP and route class. The client IP is the TCP peer: the
// gateway is the edge in this deployment, so X-Forwarded-For is not trusted (a load balancer in
// front would need it, with the LB's address as the only trusted proxy).
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	classes map[string][2]float64 // class -> rate per second, burst
}

func newLimiter(general float64) *limiter {
	l := &limiter{buckets: map[string]*bucket{}, classes: map[string][2]float64{
		"general": {general, general * 2},
		"advisor": {0.5, 6}, // each chat turn costs LLM calls
		"upload":  {1, 10},  // OCR is CPU-heavy
		"admin":   {general, general * 2},
	}}
	go func() { // forget idle clients
		for range time.Tick(time.Minute) {
			l.mu.Lock()
			for k, b := range l.buckets {
				if time.Since(b.last) > 10*time.Minute {
					delete(l.buckets, k)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func classOf(r *http.Request) string {
	p := r.URL.Path
	switch {
	case strings.HasPrefix(p, "/api/advisor/chat"):
		return "advisor"
	case r.Method == http.MethodPut && strings.HasPrefix(p, "/api/kyc/applications/"), strings.HasSuffix(p, "/check"):
		return "upload"
	}
	return "general"
}

// allow takes a token; when empty it returns the wait until the next one.
func (l *limiter) allow(ip, class string) (bool, time.Duration) {
	cfg := l.classes[class]
	l.mu.Lock()
	defer l.mu.Unlock()
	k := class + "|" + ip
	b, ok := l.buckets[k]
	now := time.Now()
	if !ok {
		b = &bucket{tokens: cfg[1], last: now}
		l.buckets[k] = b
	}
	b.tokens = math.Min(cfg[1], b.tokens+now.Sub(b.last).Seconds()*cfg[0])
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / cfg[0] * float64(time.Second))
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (l *limiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		if ok, wait := l.allow(clientIP(r), classOf(r)); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			svc.JSON(w, http.StatusTooManyRequests, map[string]string{"code": "RATE_LIMITED", "message": "too many requests; slow down and retry"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- back-office authorisation -------------------------------------------------------------

// adminOnly lists the back-office routes: review decisions, partner tasks, lead scores, order
// transitions, pipeline-wide lists and metrics. Owner routes are scoped to an unguessable lead ID
// instead (owner login/OTP is out of scope for the prototype; see docs/privacy-security-note.md).
func adminOnly(r *http.Request) bool {
	p, q := r.URL.Path, r.URL.Query()
	switch {
	case strings.HasPrefix(p, "/api/kyc/reviews"),
		strings.HasPrefix(p, "/api/scoring/tasks"), strings.HasPrefix(p, "/api/scoring/leads"),
		strings.HasSuffix(p, "/metrics"),
		r.Method == http.MethodPost && strings.HasPrefix(p, "/api/tracker/orders/") && strings.HasSuffix(p, "/transitions"):
		return true
	case r.Method == http.MethodGet && (p == "/api/tracker/orders" || p == "/api/notify/messages") && q.Get("leadId") == "":
		return true
	}
	return false
}

// requireAdmin checks X-Admin-Token on back-office routes in constant time.
func requireAdmin(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions && adminOnly(r) {
			got := r.Header.Get("X-Admin-Token")
			if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				svc.JSON(w, http.StatusUnauthorized, map[string]string{"code": "ADMIN_TOKEN_REQUIRED", "message": "back-office route: send a valid X-Admin-Token"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
