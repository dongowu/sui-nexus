package gateway

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sui-nexus/gateway/pkg/hmac"
)

func HMACAuth(signer *hmac.Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		signature := c.GetHeader("X-Signature")
		timestampStr := c.GetHeader("X-Timestamp")

		if apiKey == "" || signature == "" || timestampStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authentication headers",
			})
			return
		}

		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Invalid timestamp format",
			})
			return
		}

		if !signer.ValidateTimestamp(timestamp) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Timestamp outside allowed window",
			})
			return
		}

		c.Set("api_key", apiKey)
		c.Set("auth_signature", signature)
		c.Set("auth_timestamp", timestamp)
		c.Next()
	}
}

func RateLimit(limit int) gin.HandlerFunc {
	type bucket struct {
		tokens    int
		lastReset time.Time
	}

	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.ClientIP()
		}

		mu.Lock()
		b, exists := buckets[apiKey]
		if !exists {
			b = &bucket{tokens: limit, lastReset: time.Now()}
			buckets[apiKey] = b
		}

		if time.Since(b.lastReset) > time.Minute {
			b.tokens = limit
			b.lastReset = time.Now()
		}

		if b.tokens <= 0 {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}

		b.tokens--
		mu.Unlock()
		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-API-Key, X-Signature, X-Timestamp, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
