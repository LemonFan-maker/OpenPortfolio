package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"portfolio-hub/apps/server/internal/config"
)

func TestRuntimeRedisFailureFallsBackAndRecovers(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()

	store := NewStore(config.Config{RedisAddr: addr})
	if !store.Enabled() || store.Degraded() {
		t.Fatalf("expected a healthy redis store")
	}

	ctx := context.Background()
	if err := store.SetJSON(ctx, "cache:before", map[string]int{"v": 1}, time.Minute); err != nil {
		t.Fatalf("set before failure: %v", err)
	}

	fakeNow := time.Now()
	store.now = func() time.Time { return fakeNow }

	mr.Close()

	if err := store.SetJSON(ctx, "cache:during", map[string]int{"v": 2}, time.Minute); err != nil {
		t.Fatalf("set during failure should use memory: %v", err)
	}
	if !store.Degraded() {
		t.Fatalf("expected store to be degraded after a redis failure")
	}

	var got map[string]int
	ok, err := store.GetJSON(ctx, "cache:during", &got)
	if err != nil || !ok || got["v"] != 2 {
		t.Fatalf("memory fallback read = (%v, %v, %#v), want hit with 2", ok, err, got)
	}

	if _, err := store.DeletePatternCount(ctx, "cache:during"); err != nil {
		t.Fatalf("delete during failure: %v", err)
	}
	ok, err = store.GetJSON(ctx, "cache:during", &got)
	if err != nil {
		t.Fatalf("read after memory delete: %v", err)
	}
	if ok {
		t.Fatalf("expected memory key to be deleted")
	}

	restarted := miniredis.NewMiniRedis()
	if err := restarted.StartAddr(addr); err != nil {
		t.Fatalf("restart miniredis on %s: %v", addr, err)
	}
	t.Cleanup(restarted.Close)
	restarted.Set("cache:during", `{"v":3}`)
	restarted.Set("cache:survivor", `{"v":4}`)

	fakeNow = fakeNow.Add(redisDegradedTTL + time.Second)

	ok, err = store.GetJSON(ctx, "cache:survivor", &got)
	if err != nil || !ok || got["v"] != 4 {
		t.Fatalf("redis read after recovery = (%v, %v, %#v), want hit with 4", ok, err, got)
	}
	if store.Degraded() {
		t.Fatalf("expected store to recover after a successful redis operation")
	}
	if restarted.Exists("cache:during") {
		t.Fatalf("pending delete pattern was not replayed after recovery")
	}
	if !restarted.Exists("cache:survivor") {
		t.Fatalf("unrelated key was deleted during replay")
	}
}
