package web

import (
	"net/http"
	"time"
)

const metricsHandlerTimeout = 5 * time.Second

func servingMetricsHandler(handler http.Handler) http.Handler {
	return newServingMetricsHandler(handler, metricsHandlerTimeout)
}

func newServingMetricsHandler(handler http.Handler, timeout time.Duration) http.Handler {
	if handler == nil {
		return http.NotFoundHandler()
	}
	if timeout <= 0 {
		timeout = metricsHandlerTimeout
	}
	active := make(chan struct{}, 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case active <- struct{}{}:
		default:
			http.Error(w, "metrics scrape already in progress", http.StatusServiceUnavailable)
			return
		}

		timed := http.TimeoutHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() { <-active }()
			handler.ServeHTTP(w, r)
		}), timeout, "metrics scrape timed out\n")
		timed.ServeHTTP(w, r)
	})
}
