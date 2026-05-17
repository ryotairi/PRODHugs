package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// wsUniqueUserCount tracks the number of distinct authenticated WebSocket users.
	wsUniqueUserCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ws_unique_user_count",
		Help: "Number of distinct authenticated WebSocket users.",
	})
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed.",
		},
	)
)

// Register creates a new prometheus registry, registers all metrics, and returns
// an http.Server ready to serve /metrics on the given addr.
func Register(addr string, pool *pgxpool.Pool) *http.Server {
	reg := prometheus.NewRegistry()

	// Standard Go runtime + process collectors
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// HTTP metrics
	reg.MustRegister(httpRequestsTotal)
	reg.MustRegister(httpRequestDuration)
	reg.MustRegister(httpRequestsInFlight)

	// WebSocket metrics
	reg.MustRegister(wsUniqueUserCount)

	// DB pool metrics (using GaugeFuncs that read live stats from pgxpool)
	if pool != nil {
		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_total_conns",
			Help: "Total number of connections in the pool.",
		}, func() float64 { return float64(pool.Stat().TotalConns()) }))

		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_idle_conns",
			Help: "Number of idle connections in the pool.",
		}, func() float64 { return float64(pool.Stat().IdleConns()) }))

		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_max_conns",
			Help: "Maximum number of connections allowed in the pool.",
		}, func() float64 { return float64(pool.Stat().MaxConns()) }))

		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_acquire_count_total",
			Help: "Cumulative count of successful acquires from the pool.",
		}, func() float64 { return float64(pool.Stat().AcquireCount()) }))

		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_constructing_conns",
			Help: "Number of connections currently being constructed.",
		}, func() float64 { return float64(pool.Stat().ConstructingConns()) }))
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

// SetWSUniqueUserCount updates the WebSocket unique user count gauge.
func SetWSUniqueUserCount(count int) {
	wsUniqueUserCount.Set(float64(count))
}

// Middleware returns an Echo middleware that instruments HTTP requests with
// request count, duration, and in-flight gauges.
func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httpRequestsInFlight.Inc()
			start := time.Now()

			err := next(c)

			httpRequestsInFlight.Dec()

			status := c.Response().Status
			path := c.Path() // use the route template, not the actual URI, to avoid high cardinality
			if path == "" {
				path = "unknown"
			}
			method := c.Request().Method

			httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
			httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())

			return err
		}
	}
}

// StartServer starts the metrics HTTP server in the background and returns.
// Errors other than http.ErrServerClosed are logged.
func StartServer(srv *http.Server, l *slog.Logger) {
	go func() {
		l.Info(fmt.Sprintf("Metrics server listening on %s", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("metrics server error", "error", err)
		}
	}()
}

// StopServer gracefully shuts down the metrics HTTP server.
func StopServer(ctx context.Context, srv *http.Server) error {
	return srv.Shutdown(ctx)
}
