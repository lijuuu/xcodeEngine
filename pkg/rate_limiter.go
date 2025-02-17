package pkg

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	lastRequest     map[string]time.Time
	mu              sync.RWMutex
	cleanupInterval time.Duration
}

const RateLimit = 500 * time.Millisecond

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		lastRequest:     make(map[string]time.Time),
		cleanupInterval: 1 * time.Hour,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, lastTime := range rl.lastRequest {
			if now.Sub(lastTime) > rl.cleanupInterval {
				delete(rl.lastRequest, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipOrDomain := strings.Split(r.RemoteAddr, ":")[0]
		if ipOrDomain == "::1" || ipOrDomain == "127.0.0.1" {
			ipOrDomain = "localhost"
		}

		log.Println("Request from:", ipOrDomain)

		rl.mu.RLock()
		lastRequestTime, exists := rl.lastRequest[ipOrDomain]
		rl.mu.RUnlock()

		if exists && time.Since(lastRequestTime) < RateLimit {
			log.Printf("Rate limit exceeded for %s", ipOrDomain)
			http.Error(w, "Rate limit exceeded, try again later", http.StatusTooManyRequests)
			return
		}

		rl.mu.Lock()
		rl.lastRequest[ipOrDomain] = time.Now()
		rl.mu.Unlock()
		log.Printf("Request accepted for %s", ipOrDomain)

		next.ServeHTTP(w, r)
	})
}
