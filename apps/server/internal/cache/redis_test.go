package cache

import (
	"context"
	"testing"
	"time"

	"portfolio-hub/apps/server/internal/config"
)

func TestStoreDisabledWhenRedisAddrIsEmpty(t *testing.T) {
	store := NewStore(config.Config{RedisAddr: ""})
	if store.Enabled() {
		t.Fatalf("expected store to be disabled when REDIS_ADDR is empty")
	}
	if !store.Fallback() {
		t.Fatalf("expected memory fallback when REDIS_ADDR is empty")
	}
}

func TestStoreDisablesWhenRedisIsUnavailable(t *testing.T) {
	started := time.Now()
	store := NewStore(config.Config{RedisAddr: "127.0.0.1:0"})
	if store.Enabled() {
		t.Fatalf("expected store to be disabled when redis connection fails")
	}
	if !store.Fallback() {
		t.Fatalf("expected memory fallback when redis connection fails")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("redis unavailable fallback took %s, want under 1s", elapsed)
	}
}

func TestMemoryFallbackStoresJSONWithTTL(t *testing.T) {
	ctx := context.Background()
	store := NewStore(config.Config{RedisAddr: ""})

	input := struct {
		Name string `json:"name"`
	}{Name: "local-cache"}

	if err := store.SetJSON(ctx, "item:one", input, time.Minute); err != nil {
		t.Fatalf("SetJSON on memory fallback returned error: %v", err)
	}

	var target struct {
		Name string `json:"name"`
	}
	ok, err := store.GetJSON(ctx, "item:one", &target)
	if err != nil {
		t.Fatalf("GetJSON on memory fallback returned error: %v", err)
	}
	if !ok || target.Name != input.Name {
		t.Fatalf("GetJSON on memory fallback = (%v, %#v), want hit with %#v", ok, target, input)
	}

	if err := store.SetJSON(ctx, "item:short", input, time.Nanosecond); err != nil {
		t.Fatalf("SetJSON short ttl returned error: %v", err)
	}
	time.Sleep(time.Millisecond)
	ok, err = store.GetJSON(ctx, "item:short", &target)
	if err != nil {
		t.Fatalf("GetJSON expired item returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected expired memory fallback item to miss")
	}
}

func TestMemoryFallbackDeletesPatterns(t *testing.T) {
	ctx := context.Background()
	store := NewStore(config.Config{RedisAddr: ""})
	value := struct {
		Name string `json:"name"`
	}{Name: "delete-me"}

	_ = store.SetJSON(ctx, "item:one", value, time.Minute)
	_ = store.SetJSON(ctx, "item:two", value, time.Minute)
	_ = store.SetJSON(ctx, "other:one", value, time.Minute)

	if deleted, err := store.DeletePatternCount(ctx, "item:*"); err != nil || deleted != 2 {
		t.Fatalf("DeletePatternCount on memory fallback = (%d, %v), want (2, nil)", deleted, err)
	}
}

func TestMemoryFallbackListsJSONRecords(t *testing.T) {
	ctx := context.Background()
	store := NewStore(config.Config{RedisAddr: ""})

	for _, name := range []string{"first", "second", "third"} {
		value := struct {
			Name string `json:"name"`
		}{Name: name}
		if err := store.PushJSON(ctx, "records", value, 2); err != nil {
			t.Fatalf("PushJSON on memory fallback returned error: %v", err)
		}
	}

	var records []struct {
		Name string `json:"name"`
	}
	if err := store.ListJSON(ctx, "records", &records, 10); err != nil {
		t.Fatalf("ListJSON on memory fallback returned error: %v", err)
	}
	if len(records) != 2 || records[0].Name != "third" || records[1].Name != "second" {
		t.Fatalf("records = %#v, want newest two records", records)
	}
}

func TestMemoryFallbackDeletesListsWithPatterns(t *testing.T) {
	ctx := context.Background()
	store := NewStore(config.Config{RedisAddr: ""})

	value := struct {
		Name string `json:"name"`
	}{Name: "record"}
	_ = store.PushJSON(ctx, "records:cache", value, 10)

	if deleted, err := store.DeletePatterns(ctx, "records:*"); err != nil || deleted != 1 {
		t.Fatalf("DeletePatterns on memory fallback = (%d, %v), want (1, nil)", deleted, err)
	}
	var records []struct {
		Name string `json:"name"`
	}
	if err := store.ListJSON(ctx, "records:cache", &records, 10); err != nil {
		t.Fatalf("ListJSON on memory fallback returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("deleted memory fallback list length = %d, want 0", len(records))
	}
}
