// Package svc is the shared service runtime: config, logging, DB, Kafka producer,
// health endpoints, request IDs, graceful shutdown.
package svc

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"udyam/pkg/config"
	"udyam/pkg/events"
	"udyam/pkg/kafkax"
	"udyam/pkg/logx"
	"udyam/pkg/pg"
)

// Options describe one service.
type Options struct {
	Name        string
	DefaultPort string
	Schema      string // Postgres schema owned by this service; empty = no database
	UsesKafka   bool   // create a producer
	Setup       func(ctx context.Context, a *App) error
	Wrap        func(http.Handler) http.Handler // optional outer handler (e.g. CORS)
}

// App is handed to Setup.
type App struct {
	Name     string
	Mux      *http.ServeMux
	Log      *slog.Logger
	DB       *pgxpool.Pool
	Producer *kafkax.Producer
	Brokers  string
	metrics  *metrics
}

// JSON writes a JSON response.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON strictly decodes a request body (max 1 MB, unknown fields rejected) and answers
// 400 on failure. Returns false when the handler should stop.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		JSON(w, 400, map[string]string{"code": "INVALID_REQUEST", "message": err.Error()})
		return false
	}
	return true
}

// Run starts the service and blocks until SIGINT/SIGTERM.
func Run(o Options) {
	log := logx.New(o.Name)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a := &App{Name: o.Name, Mux: http.NewServeMux(), Log: log, Brokers: config.String("KAFKA_BROKERS", "localhost:9092"), metrics: newMetrics()}

	if o.Schema != "" {
		db, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://udyam:udyam@localhost:5432/udyam?sslmode=disable"), o.Schema)
		if err != nil {
			log.Error("database connect failed", "err", err)
			os.Exit(1)
		}
		defer db.Close()
		a.DB = db
	}
	if o.UsesKafka {
		a.Producer = kafkax.NewProducer(a.Brokers)
		defer a.Producer.Close()
	}

	a.Mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, 200, map[string]string{"status": "ok", "service": o.Name})
	})
	a.Mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if a.DB != nil {
			c, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := a.DB.Ping(c); err != nil {
				JSON(w, 503, map[string]string{"status": "db_unavailable", "service": o.Name})
				return
			}
		}
		JSON(w, 200, map[string]string{"status": "ready", "service": o.Name})
	})
	a.Mux.HandleFunc("GET /metrics", a.metrics.handler(o.Name))
	if a.DB != nil {
		a.Gauge("db_pool_connections", func() float64 { return float64(a.DB.Stat().TotalConns()) })
	}
	a.Mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, 200, map[string]any{"service": o.Name, "time": time.Now().UTC()})
	})

	if o.Setup != nil {
		if err := o.Setup(ctx, a); err != nil {
			log.Error("setup failed", "err", err)
			os.Exit(1)
		}
	}

	port := config.String("PORT", o.DefaultPort)
	var handler http.Handler = a.Mux
	if o.Wrap != nil {
		handler = o.Wrap(handler)
	}
	srv := &http.Server{Addr: ":" + port, Handler: middleware(log, a.metrics, handler), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	log.Info("stopped")
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }

// middleware adds X-Request-Id and X-Correlation-Id (kept from the caller, so one owner journey
// shares one correlation ID across services), records metrics and writes one access log line.
func middleware(log *slog.Logger, m *metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = events.NewID()
		}
		corr := r.Header.Get("X-Correlation-Id")
		if corr == "" {
			corr = id
		}
		w.Header().Set("X-Request-Id", id)
		w.Header().Set("X-Correlation-Id", corr)
		r.Header.Set("X-Request-Id", id)
		r.Header.Set("X-Correlation-Id", corr)
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(rec, r)
		d := time.Since(start)
		if r.URL.Path != "/metrics" && r.URL.Path != "/healthz" {
			m.observe(r.Method, r.URL.Path, rec.status, d)
		}
		attrs := []any{"method", r.Method, "path", r.URL.Path, "status", rec.status, "ms", d.Milliseconds(), "requestId", id, "correlationId", corr}
		if lead := r.URL.Query().Get("leadId"); lead != "" {
			attrs = append(attrs, "leadId", lead)
		}
		log.Info("http", attrs...)
	})
}
