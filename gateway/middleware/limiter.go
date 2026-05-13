package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

func RateLimitMiddleware(redisClient *redis.Client, limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			key := fmt.Sprintf("rate_limit:ip:%s", ip)
			ctx := r.Context()

			val, err := redisClient.Incr(ctx, key).Result()
			if err != nil {
				fmt.Printf("redis rate limiter failed: %v\n", err)
				next.ServeHTTP(w, r)
				return
			}

			if val == 1 {
				redisClient.Expire(ctx, key, window)
			}

			if val > limit {
				http.Error(w, "Too may requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
