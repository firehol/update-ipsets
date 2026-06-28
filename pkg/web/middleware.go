package web

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type clientRateLimiter struct {
	mu        sync.Mutex
	ttl       time.Duration
	burst     int
	every     time.Duration
	lastPrune time.Time
	clients   map[string]*visitor
}

func newClientRateLimiter(limit int, window time.Duration) *clientRateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	every := window / time.Duration(limit)
	if every <= 0 {
		every = time.Nanosecond
	}
	return &clientRateLimiter{
		ttl:     window,
		burst:   limit,
		every:   every,
		clients: map[string]*visitor{},
	}
}

func (l *clientRateLimiter) Allow(key string, now time.Time) bool {
	if l == nil || key == "" {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastPrune.IsZero() || now.Sub(l.lastPrune) >= l.ttl {
		for ip, v := range l.clients {
			if now.Sub(v.lastSeen) >= l.ttl {
				delete(l.clients, ip)
			}
		}
		l.lastPrune = now
	}
	v := l.clients[key]
	if v == nil {
		v = &visitor{limiter: rate.NewLimiter(rate.Every(l.every), l.burst)}
		l.clients[key] = v
	}
	v.lastSeen = now
	return v.limiter.AllowN(now, 1)
}

func rateLimitMiddleware(resolver *clientIPResolver, next http.Handler) http.Handler {
	limiter := newClientRateLimiter(240, time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if !strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}
		if limiter.Allow(resolver.clientIP(r), time.Now()) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	})
}

func recoverMiddleware(logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if logger != nil {
					logger.Error(
						"panic in http handler",
						"method", r.Method,
						"path", r.URL.Path,
						"client", resolver.clientIP(r),
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
				}
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestPathLogger(_ *slog.Logger) *slog.Logger {
	return plainLivenessLogger()
}

func wrapAdminAuth(opts Options, handler http.HandlerFunc) http.HandlerFunc {
	switch opts.AdminAuthMode {
	case AdminAuthModeDisabled:
		return handler
	default:
		return basicAuth(handler)
	}
}

func basicAuth(handler http.HandlerFunc) http.HandlerFunc {
	user := os.Getenv("UPDATE_IPSETS_ADMIN_USER")
	pass := os.Getenv("UPDATE_IPSETS_ADMIN_PASSWORD")
	return func(w http.ResponseWriter, r *http.Request) {
		if user == "" || pass == "" {
			http.Error(w, "admin authentication is not configured", http.StatusServiceUnavailable)
			return
		}
		gotUser, gotPass, ok := r.BasicAuth()
		// Use constant-time comparison to prevent timing side-channel attacks.
		userMatch := subtle.ConstantTimeCompare([]byte(gotUser), []byte(user))
		passMatch := subtle.ConstantTimeCompare([]byte(gotPass), []byte(pass))
		if !ok || userMatch&passMatch != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="update-ipsets admin"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}
}

// statusCapture wraps http.ResponseWriter to capture the status code.
type statusCapture struct {
	http.ResponseWriter
	code int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.code = code
	sc.ResponseWriter.WriteHeader(code)
}

// logMiddleware logs server errors (5xx) and API client errors (4xx).
func logMiddleware(logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sc := &statusCapture{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sc, r)
		if sc.code >= 500 {
			logger.Error("http server error", "method", r.Method, "path", r.URL.Path, "status", sc.code, "client", resolver.clientIP(r))
		} else if sc.code >= 400 && strings.HasPrefix(r.URL.Path, "/api/") {
			logger.Warn("http client error", "method", r.Method, "path", r.URL.Path, "status", sc.code, "client", resolver.clientIP(r))
		}
	})
}

type clientIPResolver struct {
	trustProxy      bool
	trustCloudflare bool
}

func (r *clientIPResolver) clientIP(req *http.Request) string {
	if r.trustCloudflare {
		if value := strings.TrimSpace(req.Header.Get("CF-Connecting-IP")); value != "" {
			if ip := net.ParseIP(value); ip != nil {
				return ip.String()
			}
		}
	}
	if r.trustProxy {
		if value := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); value != "" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
			if ip := net.ParseIP(value); ip != nil {
				return ip.String()
			}
		}
		if value := strings.TrimSpace(req.Header.Get("X-Real-IP")); value != "" {
			if ip := net.ParseIP(value); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err == nil {
		return host
	}
	return req.RemoteAddr
}
