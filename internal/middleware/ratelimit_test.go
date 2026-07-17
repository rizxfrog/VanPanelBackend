package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func requireRedis(t *testing.T, rdb *redis.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis 不可用，跳过速率限制测试: %v", err)
	}
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 14})
	requireRedis(t, rdb)
	defer rdb.FlushDB(context.Background())
	rl := NewRateLimiter(rdb, nil)
	rl.AddLimit("/api/test", RateLimit{Window: time.Minute, MaxCalls: 5})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/api/test", func(c *gin.Context) { c.String(200, "ok") })

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test", nil)
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("request %d should be allowed, got %d", i, w.Code)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	requireRedis(t, rdb)
	defer rdb.FlushDB(context.Background())
	rl := NewRateLimiter(rdb, nil)
	rl.AddLimit("/api/test2", RateLimit{Window: time.Minute, MaxCalls: 2})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/api/test2", func(c *gin.Context) { c.String(200, "ok") })

	// 前 2 次应该通过
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test2", nil)
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	// 第 3 次应该被限制
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test2", nil)
	router.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Fatalf("request should be rate limited, got %d", w.Code)
	}
}
