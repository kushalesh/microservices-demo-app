// api-gateway: thin HTTP front door that fans out to product-service & notification-service.
// - structured logs (zap)
// - Prometheus metrics on /metrics
// - graceful shutdown on SIGTERM
// - readiness/liveness probes
package main

import (
"context"
"encoding/json"
"errors"
"net/http"
"os"
"os/signal"
"syscall"
"time"

"github.com/go-chi/chi/v5"
"github.com/go-chi/chi/v5/middleware"
"github.com/prometheus/client_golang/prometheus"
"github.com/prometheus/client_golang/prometheus/promauto"
"github.com/prometheus/client_golang/prometheus/promhttp"
"go.uber.org/zap"
)

var (
httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
Name: "http_requests_total",
Help: "HTTP requests handled by the api-gateway, partitioned by route and status.",
}, []string{"route", "status"})
httpLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
Name:    "http_request_duration_seconds",
Help:    "Latency distribution of HTTP requests.",
Buckets: prometheus.DefBuckets,
}, []string{"route"})
)

func main() {
logger, _ := zap.NewProduction()
defer logger.Sync()

productURL := getenv("PRODUCT_SERVICE_URL", "http://product-service:3000")
notifyURL := getenv("NOTIFICATION_SERVICE_URL", "http://notification-service:5000")

r := chi.NewRouter()
r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
r.Use(metricsMiddleware)

r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
})
r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
})

r.Handle("/metrics", promhttp.Handler())

r.Get("/api/v1/products", proxyJSON(productURL+"/products", logger))
r.Post("/api/v1/notify", proxyJSON(notifyURL+"/send", logger))

srv := &http.Server{
Addr:              ":8080",
Handler:           r,
ReadHeaderTimeout: 5 * time.Second,
}

go func() {
logger.Info("api-gateway listening", zap.String("addr", srv.Addr))
if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
logger.Fatal("server error", zap.Error(err))
}
}()

stop := make(chan os.Signal, 1)
signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
<-stop
logger.Info("shutting down")

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
_ = srv.Shutdown(ctx)
}

func metricsMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
start := time.Now()
ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
next.ServeHTTP(ww, r)
route := r.URL.Path
httpRequests.WithLabelValues(route, http.StatusText(ww.Status())).Inc()
httpLatency.WithLabelValues(route).Observe(time.Since(start).Seconds())
})
}

func proxyJSON(url string, logger *zap.Logger) http.HandlerFunc {
client := &http.Client{Timeout: 5 * time.Second}
return func(w http.ResponseWriter, r *http.Request) {
req, _ := http.NewRequestWithContext(r.Context(), r.Method, url, r.Body)
req.Header = r.Header
resp, err := client.Do(req)
if err != nil {
logger.Warn("upstream error", zap.Error(err), zap.String("url", url))
http.Error(w, "upstream unavailable", http.StatusBadGateway)
return
}
defer resp.Body.Close()
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(resp.StatusCode)
var body interface{}
_ = json.NewDecoder(resp.Body).Decode(&body)
_ = json.NewEncoder(w).Encode(body)
}
}

func getenv(k, def string) string {
if v := os.Getenv(k); v != "" {
return v
}
return def
}
