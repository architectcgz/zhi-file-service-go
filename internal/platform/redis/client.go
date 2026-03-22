package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	raw *goredis.Client
}

func Open(ctx context.Context, cfg config.RedisConfig, required bool, logger *slog.Logger) (*Client, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		if required {
			return nil, errors.New("redis addr is required")
		}
		return nil, nil
	}

	raw := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := raw.Ping(ctx).Err(); err != nil {
		_ = raw.Close()
		if required {
			return nil, fmt.Errorf("ping redis: %w", err)
		}
		if logger != nil {
			logger.Warn("redis_unavailable_optional", "error", err.Error())
		}
		return nil, nil
	}

	return &Client{raw: raw}, nil
}

func (c *Client) Raw() *goredis.Client {
	if c == nil {
		return nil
	}
	return c.raw
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Ping(ctx).Err()
}

func (c *Client) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Close()
}
