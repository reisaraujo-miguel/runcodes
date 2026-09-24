/*
Package cache is a best-effort JSON cache in Redis.

Every operation degrades to a no-op when Redis is unavailable: a cache failure
must never fail a request.
*/
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/runcodes-icmc/runcodes/config"

	"github.com/redis/go-redis/v9"
)

const (
	// cacheTimeout bounds a single Redis round trip so an unresponsive cache
	// can never meaningfully delay an API request.
	cacheTimeout = 500 * time.Millisecond

	// cacheCooldown is how long the layer stops talking to Redis after a
	// failure, so a down cache does not add a timeout to every request.
	cacheCooldown = 30 * time.Second

	// OfferingTTL is how long an offering is cached.
	OfferingTTL = 30 * time.Second
	// ExercisesTTL is how long an offering's exercise list is cached.
	ExercisesTTL = 30 * time.Second
	// AllowedFileTypesTTL is how long the allowed file type catalog is cached.
	AllowedFileTypesTTL = 5 * time.Minute

	// SettingsTTL is how long the platform settings are cached. They are read on
	// the public login page, so the entry is dropped on every admin write.
	SettingsTTL = 1 * time.Minute
)

/*
store wraps the lazily-initialised Redis client.
*/
type store struct {
	mu            sync.Mutex
	client        *redis.Client
	disabledUntil time.Time
	warnOnce      sync.Once
}

var cache = &store{}

/*
newRedisClient builds a Redis client from the configured address. It never
dials, so it cannot fail here.
*/
func newRedisClient() *redis.Client {
	cfg := config.Get().Redis

	return redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cacheTimeout,
		ReadTimeout:  cacheTimeout,
		WriteTimeout: cacheTimeout,
	})
}

/*
current returns the Redis client, or nil when the cache is disabled or in its
failure cooldown. It is safe for concurrent use.
*/
func (c *store) current() *redis.Client {
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
func (c *store) markFailure(err error) {
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
Ping dials Redis once at startup so configuration problems surface in the logs.
A failure is not fatal: the cache simply degrades to a no-op.
*/
func Ping(ctx context.Context) error {
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
GetJSON unmarshals the value stored at key into dst. It reports whether a
usable value was found. Any error (miss, unreachable Redis, bad payload) is a
plain false.
*/
func GetJSON(ctx context.Context, key string, dst any) bool {
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
SetJSON stores v at key as JSON for the given ttl. Failures are swallowed.
*/
func SetJSON(ctx context.Context, key string, v any, ttl time.Duration) {
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
Delete removes the given keys. Failures are swallowed.
*/
func Delete(ctx context.Context, keys ...string) {
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

/*
Keys. Kept here so producers and invalidators cannot drift apart.
*/

// KeyAllowedFileTypes caches the catalog of allowed file types.
const KeyAllowedFileTypes = "allowed_file_types"

// KeyPlatformSettings caches the platform settings shown on the login page.
const KeyPlatformSettings = "platform_settings"

// OfferingKey caches a single offering.
func OfferingKey(id int64) string {
	return "offering:" + strconv.FormatInt(id, 10)
}

// OfferingExercisesKey caches an offering's exercise list.
func OfferingExercisesKey(id int64) string {
	return "offering_exercises:" + strconv.FormatInt(id, 10)
}
