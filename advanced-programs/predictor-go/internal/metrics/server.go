package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	Registry            *prometheus.Registry
	RawPublished        prometheus.Counter
	NormalizedPublished prometheus.Counter
	WriterErrors        prometheus.Counter
}

func New() Metrics {
	registry := prometheus.NewRegistry()
	m := Metrics{
		Registry: registry,
		RawPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "predictor_go_raw_published_total",
			Help: "Raw payloads published by predictor-go.",
		}),
		NormalizedPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "predictor_go_normalized_published_total",
			Help: "Normalized order books published by predictor-go.",
		}),
		WriterErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "predictor_go_writer_errors_total",
			Help: "Writer errors observed by predictor-go.",
		}),
	}
	registry.MustRegister(m.RawPublished, m.NormalizedPublished, m.WriterErrors)
	return m
}

func Handler(registry *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	return mux
}

func Serve(ctx context.Context, addr string, registry *prometheus.Registry) error {
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           Handler(registry),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
