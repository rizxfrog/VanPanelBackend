package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimit 速率限制配置
type RateLimit struct {
	Window   time.Duration
	MaxCalls int
}

// RateLimiter 速率限制器
type RateLimiter struct {
	redis  *redis.Client
	limits map[string]RateLimit
	logger *zap.Logger
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(redis *redis.Client, logger *zap.Logger) *RateLimiter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RateLimiter{
		redis:  redis,
		limits: make(map[string]RateLimit),
		logger: logger,
	}
}

// AddLimit 添加路由限制配置
func (rl *RateLimiter) AddLimit(path string, limit RateLimit) {
	rl.limits[path] = limit
}

// Middleware 返回 Gin 中间件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, ok := rl.limits[c.Request.URL.Path]
		if !ok {
			c.Next()
			return
		}

		key := fmt.Sprintf("ratelimit:%s:%s", c.Request.URL.Path, c.ClientIP())
		allowed, err := rl.checkLimit(c.Request.Context(), key, limit)
		if err != nil {
			rl.logger.Warn("速率限制检查失败，放行", zap.Error(err))
			c.Next()
			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}

		c.Next()
	}
}

const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local max_calls = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)
if count >= max_calls then
    return 0
end
redis.call('ZADD', key, now, ARGV[4])
redis.call('EXPIRE', key, window / 1000)
return 1
`

func (rl *RateLimiter) checkLimit(ctx context.Context, key string, limit RateLimit) (bool, error) {
	now := time.Now().UnixMilli()
	windowMs := limit.Window.Milliseconds()
	memberID := fmt.Sprintf("%d:%d", now, time.Now().UnixNano())
	result, err := rl.redis.Eval(ctx, slidingWindowScript, []string{key}, now, windowMs, limit.MaxCalls, memberID).Int()
	if err != nil {
		return true, err
	}
	return result == 1, nil
}
