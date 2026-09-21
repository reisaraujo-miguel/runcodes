package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisAddrEnv     = "RUNCODES_REDIS_ADDR"
	redisPasswordEnv = "RUNCODES_REDIS_PASSWORD"
	redisDBEnv       = "RUNCODES_REDIS_DB"

	defaultRedisAddr = "redis:6379"

	// cacheTimeout bounds a single Redis round trip so an unresponsive cache
	// can never meaningfully delay an API request.
	cacheTimeout = 500 * time.Millisecond

	// cacheCooldown is how long the layer stops talking to Redis after a
	// failure, so a down cache does not add a timeout to every request.
	cacheCooldown = 30 * time.Second

	// CacheOfferingTTL is how long an offering is cached.
	CacheOfferingTTL = 30 * time.Second
	// CacheExercisesTTL is how long an offering's exercise list is cached.
	CacheExercisesTTL = 30 * time.Second
	// CacheAllowedFileTypesTTL is how long the allowed file type catalog is cached.
	CacheAllowedFileTypesTTL = 5 * time.Minute
)

/*
cacheStore wraps the lazily-initialised Redis client. Every operation degrades
to a no-op when Redis is unavailable: a cache failure must never fail a request.
*/
type cacheStore struct {
	mu            sync.Mutex
	client        *redis.Client
	disabledUntil time.Time
	warnOnce      sync.Once
}

var cache = &cacheStore{}

/*
newRedisClient builds a Redis client from the RUNCODES_REDIS_* environment
variables. It never dials, so it cannot fail here.
*/
func newRedisClient() *redis.Client {
	addr := os.Getenv(redisAddrEnv)
	if addr == "" {
		addr = defaultRedisAddr
	}

	db := 0
	if raw := os.Getenv(redisDBEnv); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			db = parsed
		} else {
			slog.Warn("invalid redis db, using default",
				slog.String("value", raw),
				slog.String("error", err.Error()),
			)
		}
	}

	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     os.Getenv(redisPasswordEnv),
		DB:           db,
		DialTimeout:  cacheTimeout,
		ReadTimeout:  cacheTimeout,
		WriteTimeout: cacheTimeout,
	})
}

/*
client returns the Redis client, or nil when the cache is disabled or in its
failure cooldown. It is safe for concurrent use.
*/
func (c *cacheStore) current() *redis.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		c.client = newRedisClient()
	}
	if time.Now().Before(c.disabledUntil) {
		return nil
	}

	return c.client
}

/*
markFailure records that Redis is unreachable. The warning is logged once; the
layer then goes silent for the cooldown so it cannot slow requests down.
*/
func (c *cacheStore) markFailure(err error) {
	c.warnOnce.Do(func() {
		slog.Warn("redis cache unavailable, degrading to no-op",
			slog.String("error", err.Error()),
		)
	})

	c.mu.Lock()
	c.disabledUntil = time.Now().Add(cacheCooldown)
	c.mu.Unlock()
}

/*
PingCache dials Redis once at startup so configuration problems surface in the
logs. A failure is not fatal: the cache simply degrades to a no-op.
*/
func PingCache(ctx context.Context) error {
	client := cache.current()
	if client == nil {
		return errors.New("redis cache is disabled")
	}

	pingCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		cache.markFailure(err)
		return err
	}

	return nil
}

/*
CacheGetJSON unmarshals the value stored at key into dst. It reports whether a
usable value was found. Any error (miss, unreachable Redis, bad payload) is a
plain false.
*/
func CacheGetJSON(ctx context.Context, key string, dst any) bool {
	client := cache.current()
	if client == nil {
		return false
	}

	getCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()

	raw, err := client.Get(getCtx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			cache.markFailure(err)
		}
		return false
	}

	if err := json.Unmarshal(raw, dst); err != nil {
		slog.WarnContext(ctx, "discarding malformed cache entry",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return false
	}

	return true
}

/*
CacheSetJSON stores v at key as JSON for the given ttl. Failures are swallowed.
*/
func CacheSetJSON(ctx context.Context, key string, v any, ttl time.Duration) {
	client := cache.current()
	if client == nil {
		return
	}

	raw, err := json.Marshal(v)
	if err != nil {
		slog.ErrorContext(ctx, "failed to encode cache value",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return
	}

	setCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()

	if err := client.Set(setCtx, key, raw, ttl).Err(); err != nil {
		cache.markFailure(err)
	}
}

/*
CacheDelete removes the given keys. Failures are swallowed.
*/
func CacheDelete(ctx context.Context, keys ...string) {
	if len(keys) == 0 {
		return
	}

	client := cache.current()
	if client == nil {
		return
	}

	delCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()

	if err := client.Del(delCtx, keys...).Err(); err != nil {
		cache.markFailure(err)
	}
}

// Cache keys. Kept here so producers and invalidators cannot drift apart.
const (
	cacheKeyAllowedFileTypes = "allowed_file_types"

	cacheKeyOfferingPrefix  = "offering:"
	cacheKeyExercisesPrefix = "offering_exercises:"
)

func cacheKeyOffering(id int64) string {
	return cacheKeyOfferingPrefix + strconv.FormatInt(id, 10)
}

func cacheKeyOfferingExercises(id int64) string {
	return cacheKeyExercisesPrefix + strconv.FormatInt(id, 10)
}
