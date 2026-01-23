package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"coderefinery/internal/config"

	"github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type HybridCache struct {
	memory *cache.Cache
	redis  *redis.Client
	cfg    config.CacheConfig
}

func NewHybridCache(cfg config.CacheConfig) (*HybridCache, error) {
	if !cfg.Enabled {
		return &HybridCache{cfg: cfg}, nil
	}

	// 1. Redis Client (L2 Cache)
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(opts)

	// Ping Test
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("Redis not available, falling back to memory-only")
		// Wir geben keinen Fehler zurück, damit die App auch ohne Redis startet
		rdb = nil
	}

	// 2. Memory Cache (L1 Cache)
	// Standard expiration: 5 min, Cleanup alle 10 min
	mem := cache.New(5*time.Minute, 10*time.Minute)

	return &HybridCache{
		memory: mem,
		redis:  rdb,
		cfg:    cfg,
	}, nil
}

func (c *HybridCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	if !c.cfg.Enabled {
		return false, nil
	}

	// 1. L1 Check (Memory)
	if val, found := c.memory.Get(key); found {

		if b, ok := val.([]byte); ok {
			if err := json.Unmarshal(b, dest); err == nil {
				return true, nil
			}
		}
	}

	// 2. L2 Check (Redis)
	if c.redis != nil {
		val, err := c.redis.Get(ctx, key).Bytes()
		if err == nil {
			// Found in Redis -> Promote to Memory (L1)
			c.memory.Set(key, val, cache.DefaultExpiration)

			if err := json.Unmarshal(val, dest); err != nil {
				return false, err
			}
			return true, nil
		} else if !errors.Is(err, redis.Nil) {
			// Echter Redis Fehler (nicht nur "key missing")
			log.Warn().Err(err).Msg("Redis get error")
		}
	}

	return false, nil
}

func (c *HybridCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !c.cfg.Enabled {
		return nil
	}

	if ttl == 0 {
		ttl = c.cfg.TTL
	}

	// Nach JSON serialisieren
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// 1. Set L1
	c.memory.Set(key, data, ttl)

	// 2. Set L2
	if c.redis != nil {
		// Async, damit wir den User nicht blockieren?
		// Fürs Erste synchron, ist sicherer.
		if err := c.redis.Set(ctx, key, data, ttl).Err(); err != nil {
			log.Warn().Err(err).Msg("Redis set error")
		}
	}

	return nil
}
