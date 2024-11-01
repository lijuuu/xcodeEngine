package pkg

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	lastRequest map[string]time.Time
	mu          sync.Mutex
}

const RateLimit = 500 * time.Millisecond

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		lastRequest: make(map[string]time.Time),
	}
}

func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipOrDomain := strings.Split(r.RemoteAddr, ":")[0]
		if ipOrDomain == "::1" || ipOrDomain == "127.0.0.1" {
			ipOrDomain = "localhost"
		}

		log.Println("Request from:", ipOrDomain)

		rl.mu.Lock()
		defer rl.mu.Unlock()

		if lastRequestTime, exists := rl.lastRequest[ipOrDomain]; exists {
			if time.Since(lastRequestTime) < RateLimit {
				log.Printf("Rate limit exceeded for %s", ipOrDomain)
				http.Error(w, "Rate limit exceeded, try again later", http.StatusTooManyRequests)
				return
			}
		}

		rl.lastRequest[ipOrDomain] = time.Now()
		log.Printf("Request accepted for %s", ipOrDomain)

		next.ServeHTTP(w, r)
	})
}
