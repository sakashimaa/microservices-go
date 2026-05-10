package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
}

type CacheRedis struct {
	redis *redis.Client
}

func NewTokenCache(redis *redis.Client) TokenCache {
	return &CacheRedis{
		redis: redis,
	}
}

func (c CacheRedis) Get(ctx context.Context, key string) (string, error) {
	return c.redis.Get(ctx, key).Result()
}

func (c CacheRedis) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return c.redis.Set(ctx, key, value, expiration).Err()
}
