package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServerConfig holds configuration for the metrics HTTP server.
type MetricsServerConfig struct {
	// Enabled indicates whether the metrics server should run.
	Enabled bool

	// Port is the port to listen on.
	Port int

	// Path is the path to serve metrics on.
	Path string

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration
}

// DefaultMetricsServerConfig returns the default metrics server configuration.
func DefaultMetricsServerConfig() MetricsServerConfig {
	return MetricsServerConfig{
		Enabled:      false,
		Port:         9090,
		Path:         "/metrics",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

// MetricsServer provides an HTTP server for Prometheus metrics.
type MetricsServer struct {
	config MetricsServerConfig
	server *http.Server
}

// NewMetricsServer creates a new metrics server.
func NewMetricsServer(config MetricsServerConfig) *MetricsServer {
	return &MetricsServer{
		config: config,
	}
}

// Start starts the metrics HTTP server.
func (s *MetricsServer) Start() error {
	if !s.config.Enabled {
		return nil
	}

	// Initialize metrics if not already done
	InitMetrics()

	mux := http.NewServeMux()
	mux.Handle(s.config.Path, promhttp.Handler())

	// Add a simple health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash - metrics are non-critical
			fmt.Printf("metrics server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the metrics server.
func (s *MetricsServer) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	return s.server.Shutdown(ctx)
}

// Addr returns the server address.
func (s *MetricsServer) Addr() string {
	if s.server == nil {
		return ""
	}
	return s.server.Addr
}
