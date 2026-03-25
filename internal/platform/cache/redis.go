package cache

import (
	"context"
	"fmt"

	"skilljudge/backend/internal/config"

	"github.com/redis/go-redis/v9"
)

func NewRedis(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed (%s, db=%d): %w", cfg.Addr, cfg.DB, err)
	}

	return client, nil
}
