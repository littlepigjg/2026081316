package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"process-guardian/pkg/logger"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		logger.Infof("%s %s %d %d bytes %v",
			r.Method, r.URL.Path, rec.status, rec.size, duration)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("PANIC: %v\n%s", err, debug.Stack())

				http.Error(w, "Internal Server Error", http.StatusInternalServerError)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "internal_server_error",
					"message": fmt.Sprintf("unexpected error: %v", err),
				})
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func ContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" {
			if ct := r.Header.Get("Content-Type"); ct != "" {
				if len(ct) > 16 && ct[:16] == "application/json" {
				} else if ct != "application/json" {
					w.Header().Set("Content-Type", "application/json")
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func RateLimitMiddleware(requestsPerSecond int) func(http.Handler) http.Handler {
	type bucket struct {
		tokens   int
		lastTime time.Time
	}

	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			mu.Lock()
			b, exists := buckets[ip]
			if !exists {
				b = &bucket{tokens: requestsPerSecond, lastTime: time.Now()}
				buckets[ip] = b
			}
			now := time.Now()
			elapsed := now.Sub(b.lastTime).Seconds()
			b.tokens += int(elapsed * float64(requestsPerSecond))
			if b.tokens > requestsPerSecond {
				b.tokens = requestsPerSecond
			}
			b.lastTime = now

			if b.tokens <= 0 {
				mu.Unlock()
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "rate_limit_exceeded",
					"message": "too many requests, please try again later",
				})
				return
			}
			b.tokens--
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

type MetricsCollector struct {
	mu             sync.Mutex
	TotalRequests  int64
	TotalDuration  time.Duration
	StartedAt      time.Time
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		StartedAt: time.Now(),
	}
}

func (mc *MetricsCollector) Record(duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.TotalRequests++
	mc.TotalDuration += duration
}

func (mc *MetricsCollector) Snapshot() (int64, time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.TotalRequests, mc.TotalDuration
}

func MetricsMiddleware(collector *MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)
			collector.Record(duration)
		})
	}
}