// Command gateway is the single entry point for web and mobile: reverse proxy to services,
// CORS, request IDs, aggregated health, and the Phase 0 ping publisher.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"udyam/pkg/config"
	"udyam/pkg/events"
	"udyam/pkg/svc"
)

func cors(next http.Handler) http.Handler {
	origins := config.String("CORS_ORIGINS", "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	upstreams := map[string]string{
		"registry": config.String("REGISTRY_URL", "http://registry:8081"),
		"rules":    config.String("RULES_URL", "http://rules:8082"),
		"quote":    config.String("QUOTE_URL", "http://quote:8083"),
		"kyc":      config.String("KYC_URL", "http://kyc:8084"),
		"tracker":  config.String("TRACKER_URL", "http://tracker:8085"),
		"notify":   config.String("NOTIFY_URL", "http://notify:8086"),
		"scoring":  config.String("SCORING_URL", "http://scoring:8087"),
		"advisor":  config.String("ADVISOR_URL", "http://advisor:8090"),
	}

	svc.Run(svc.Options{
		Name: "gateway", DefaultPort: "8080", UsesKafka: true, Wrap: cors,
		Setup: func(ctx context.Context, a *svc.App) error {
			for name, raw := range upstreams {
				u, err := url.Parse(raw)
				if err != nil {
					return err
				}
				prefix := "/api/" + name
				a.Mux.Handle(prefix+"/", http.StripPrefix(prefix, httputil.NewSingleHostReverseProxy(u)))
			}

			// Aggregated health of every upstream.
			a.Mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
				var mu sync.Mutex
				var wg sync.WaitGroup
				result := map[string]string{"gateway": "ok"}
				client := &http.Client{Timeout: 2 * time.Second}
				for name, raw := range upstreams {
					wg.Add(1)
					go func(name, raw string) {
						defer wg.Done()
						state := "down"
						if resp, err := client.Get(strings.TrimRight(raw, "/") + "/healthz"); err == nil {
							if resp.StatusCode == 200 {
								state = "ok"
							}
							resp.Body.Close()
						}
						mu.Lock()
						result[name] = state
						mu.Unlock()
					}(name, raw)
				}
				wg.Wait()
				overall := "ok"
				for _, s := range result {
					if s != "ok" {
						overall = "degraded"
					}
				}
				svc.JSON(w, 200, map[string]any{"status": overall, "services": result})
			})

			// Phase 0 smoke test: publish system.ping; notify consumes it and stores it.
			a.Mux.HandleFunc("POST /api/system/ping", func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Message string `json:"message"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.Message == "" {
					body.Message = "ping"
				}
				env, err := events.New(events.TopicSystemPing, "", r.Header.Get("X-Request-Id"), events.PingPayload{Message: body.Message})
				if err != nil {
					svc.JSON(w, 500, map[string]string{"error": err.Error()})
					return
				}
				if err := a.Producer.Publish(r.Context(), events.TopicSystemPing, env); err != nil {
					svc.JSON(w, 502, map[string]string{"error": "kafka publish failed: " + err.Error()})
					return
				}
				svc.JSON(w, 202, map[string]string{"id": env.ID})
			})
			return nil
		},
	})
}
