package gateway

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(1000))
	router.GET("/limited", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/limited", nil)
			req.Header.Set("X-API-Key", "agent-"+strconv.Itoa(i%5))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNoContent, rec.Code)
		}(i)
	}
	wg.Wait()
}
