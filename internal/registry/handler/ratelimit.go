package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter returns a Gin middleware that enforces per-IP token-bucket
// rate limiting. rps is the steady-state requests per second; burst is the
// maximum burst size. Stale entries are cleaned every 5 minutes.
func RateLimiter(rps, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	limiters := make(map[string]*ipLimiter)

	// Background cleanup goroutine.
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			for ip, l := range limiters {
				if time.Since(l.lastSeen) > 10*time.Minute {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		l, ok := limiters[ip]
		if !ok {
			l = &ipLimiter{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
			limiters[ip] = l
		}
		l.lastSeen = time.Now()
		mu.Unlock()

		if !l.limiter.Allow() {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
