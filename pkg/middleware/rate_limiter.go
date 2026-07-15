package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type clientLimit struct {
	lastSeen time.Time
	tokens   int
}

// IPBasedRateLimiter limits requests by IP.
// maxTokens: maximum bucket size
// refillRate: rate at which tokens are added (per duration)
// duration: duration for the refill rate (e.g. 1 second, 1 minute)
func IPBasedRateLimiter(maxTokens int, refillRate int, duration time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	clients := make(map[string]*clientLimit)

	// Clean up old clients periodically to prevent memory leaks in long running services
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			mu.Lock()
			for ip, limit := range clients {
				if time.Since(limit.lastSeen) > 1*time.Hour {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		client, exists := clients[ip]
		now := time.Now()

		if !exists {
			clients[ip] = &clientLimit{
				lastSeen: now,
				tokens:   maxTokens - 1, // Consume one immediately
			}
			mu.Unlock()
			c.Next()
			return
		}

		// Calculate token refill based on time elapsed
		elapsed := now.Sub(client.lastSeen)
		
		// If at least one duration has elapsed, refill tokens
		refills := int(elapsed/duration) * refillRate
		if refills > 0 {
			client.tokens += refills
			if client.tokens > maxTokens {
				client.tokens = maxTokens
			}
			// Update lastSeen to avoid partial duration loss, or simply track precise floats
			client.lastSeen = now
		}

		if client.tokens <= 0 {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"status":  http.StatusTooManyRequests,
				"message": "Too many requests. Please try again later.",
			})
			return
		}

		client.tokens--
		mu.Unlock()
		c.Next()
	}
}
