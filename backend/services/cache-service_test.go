package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// useCache points the package cache at the given client for the duration of a
// test, restoring the previous store afterwards.
func useCache(t *testing.T, client *redis.Client) *cacheStore {
	t.Helper()
	previous := cache
	store := &cacheStore{client: client}
	cache = store
	t.Cleanup(func() { cache = previous })
	return store
}

func TestCacheRoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	useCache(t, redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()

	type payload struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	want := payload{A: 7, B: "x"}

	CacheSetJSON(ctx, "key", want, 5*time.Minute)

	var got payload
	if !CacheGetJSON(ctx, "key", &got) {
		t.Fatal("expected a cache hit")
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if ttl := mr.TTL("key"); ttl <= 0 || ttl > 5*time.Minute {
		t.Fatalf("unexpected ttl %v", ttl)
	}
}

func TestCacheMiss(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	useCache(t, redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	var got map[string]any
	if CacheGetJSON(context.Background(), "absent", &got) {
		t.Fatal("expected a cache miss")
	}
}

func TestCacheDiscardsMalformedEntry(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	useCache(t, redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	if err := mr.Set("bad", "not-json"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var got map[string]any
	if CacheGetJSON(context.Background(), "bad", &got) {
		t.Fatal("expected a malformed entry to be treated as a miss")
	}
}

func TestCacheDelete(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	useCache(t, redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()

	CacheSetJSON(ctx, "a", 1, time.Minute)
	CacheSetJSON(ctx, "b", 2, time.Minute)
	CacheDelete(ctx, "a", "b")

	var got int
	if CacheGetJSON(ctx, "a", &got) {
		t.Fatal("expected key a to be deleted")
	}
	if CacheGetJSON(ctx, "b", &got) {
		t.Fatal("expected key b to be deleted")
	}
}

func TestCacheDegradesWhenUnreachable(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  20 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
	})
	store := useCache(t, client)
	defer client.Close()

	ctx := context.Background()

	var got map[string]any
	if CacheGetJSON(ctx, "key", &got) {
		t.Fatal("expected a miss when redis is unreachable")
	}

	// These must not panic nor block.
	CacheSetJSON(ctx, "key", got, time.Minute)
	CacheDelete(ctx, "key")

	if store.disabledUntil.IsZero() {
		t.Fatal("expected the cache to enter its failure cooldown")
	}

	if err := PingCache(ctx); err == nil {
		t.Fatal("expected PingCache to report an error")
	}
}

func TestPingCacheSuccess(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	useCache(t, redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	if err := PingCache(context.Background()); err != nil {
		t.Fatalf("PingCache returned error: %v", err)
	}
}
