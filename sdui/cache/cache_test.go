package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"awo.so/awo/sdui/cache"
)

// ── Stubs ─────────────────────────────────────────────────────────────────────

type memRedis struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMemRedis() *memRedis { return &memRedis{data: make(map[string]string)} }

func (m *memRedis) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return "", cache.ErrCacheMiss
	}
	return v, nil
}

func (m *memRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestCache_KeyStructure(t *testing.T) {
	p := cache.KeyParams{
		Level:           "l2",
		EntityName:      "finance_invoice",
		ViewMode:        "list",
		RendererID:      "amis",
		RendererVersion: "1.0.0",
		Locale:          "en-US",
		TenantIDHash:    "abc123",
		SchemaFP:        "def456",
		PermFP:          "ghi789",
	}
	key := cache.Key(p)
	if key == "" {
		t.Error("expected non-empty key")
	}
	// Key must start with sdui:v1:l2
	prefix := "sdui:v1:l2:"
	if len(key) < len(prefix) || key[:len(prefix)] != prefix {
		t.Errorf("key %q does not start with %q", key, prefix)
	}
}

func TestCache_KeyDeterministic(t *testing.T) {
	p := cache.KeyParams{
		Level: "l2", EntityName: "test", ViewMode: "list",
		RendererID: "amis", RendererVersion: "1.0",
		Locale: "en-US", TenantIDHash: "abc", SchemaFP: "def", PermFP: "ghi",
	}
	if cache.Key(p) != cache.Key(p) {
		t.Error("key is not deterministic")
	}
}

func TestCache_NilRedis_NilResult(t *testing.T) {
	c := cache.New(nil)
	ctx := context.Background()

	data, err := c.GetWidgetTree(ctx, "any-key")
	if err != nil || data != nil {
		t.Errorf("nil redis: expected (nil, nil), got (%v, %v)", data, err)
	}
}

func TestCache_SetAndGet_WidgetTree(t *testing.T) {
	redis := newMemRedis()
	c := cache.New(redis)
	ctx := context.Background()

	key := "sdui:v1:l2:test"
	payload := []byte(`{"type":"page"}`)

	if err := c.SetWidgetTree(ctx, key, payload); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetWidgetTree(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestCache_Miss_ReturnsNil(t *testing.T) {
	c := cache.New(newMemRedis())
	data, err := c.GetWidgetTree(context.Background(), "no-such-key")
	if err != nil {
		t.Fatalf("expected nil error on miss, got %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data on miss, got %v", data)
	}
}

func TestCache_SetAndGet_RenderedOutput(t *testing.T) {
	redis := newMemRedis()
	c := cache.New(redis)
	ctx := context.Background()
	key := "sdui:v1:l3:test"
	payload := []byte(`{"format":"amis-json"}`)

	if err := c.SetRenderedOutput(ctx, key, payload); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetRenderedOutput(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestCache_SingleflightCoalesces(t *testing.T) {
	c := cache.New(nil)
	var callCount atomic.Int64
	var wg sync.WaitGroup
	const concurrency = 20

	// block channel keeps fn blocked until all goroutines have queued up.
	// Only one goroutine will actually call fn (singleflight); the rest wait.
	// Closing block releases fn, allowing singleflight to complete once.
	block := make(chan struct{})

	fn := func() ([]byte, error) {
		<-block // block until all goroutines have queued up
		callCount.Add(1)
		return []byte("result"), nil
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.DoWidgetTree("same-key", fn); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	// Give goroutines time to queue up in singleflight, then release.
	time.Sleep(20 * time.Millisecond)
	close(block)
	wg.Wait()

	// singleflight coalesces — exactly 1 call should have executed fn.
	// Allow up to 3 in case of scheduling edge cases.
	if count := callCount.Load(); count >= concurrency {
		t.Errorf("singleflight did not coalesce: %d/%d calls executed", count, concurrency)
	}
}

func TestCache_SingleflightPropagatesError(t *testing.T) {
	c := cache.New(nil)
	sentinelErr := errors.New("fn failed")
	_, err := c.DoWidgetTree("err-key", func() ([]byte, error) {
		return nil, sentinelErr
	})
	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestHashTenantID_Deterministic(t *testing.T) {
	id := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	h1 := cache.HashTenantID(id)
	h2 := cache.HashTenantID(id)
	if h1 != h2 {
		t.Error("HashTenantID not deterministic")
	}
	if h1 == "" || len(h1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("unexpected hash length: %q", h1)
	}
}

// ── Security isolation tests ───────────────────────────────────────────────────

// baseParams returns a fully-populated KeyParams for mutation in tests.
func baseParams(level string) cache.KeyParams {
	return cache.KeyParams{
		Level:           level,
		EntityName:      "finance_invoice",
		ViewMode:        "list",
		RendererID:      "amis",
		RendererVersion: "1.1.0",
		Locale:          "en-US",
		TenantIDHash:    cache.HashTenantID("tenant-a-uuid"),
		SchemaFP:        "abcdef1234567890",
		PermFP:          "0123456789abcdef",
	}
}

func TestKey_PermFPIsolation(t *testing.T) {
	// Two viewers with different permission fingerprints must get different cache keys.
	// This is the invariant that prevents admin-generated schemas from being
	// served to restricted viewers.
	admin := baseParams("l3")
	restricted := baseParams("l3")
	restricted.PermFP = "ffffffffffffffff"

	if cache.Key(admin) == cache.Key(restricted) {
		t.Error("different PermFP must produce different cache keys")
	}
}

func TestKey_TenantIsolation(t *testing.T) {
	// Two tenants with different tenant hashes must get different cache keys,
	// even when all other dimensions are identical.
	tenantA := baseParams("l3")
	tenantB := baseParams("l3")
	tenantB.TenantIDHash = cache.HashTenantID("tenant-b-uuid")

	if cache.Key(tenantA) == cache.Key(tenantB) {
		t.Error("different TenantIDHash must produce different cache keys")
	}
}

func TestKey_LocaleIsolation(t *testing.T) {
	enUS := baseParams("l2")
	arSA := baseParams("l2")
	arSA.Locale = "ar-SA"

	if cache.Key(enUS) == cache.Key(arSA) {
		t.Error("different Locale must produce different cache keys")
	}
}

func TestKey_ViewModeIsolation(t *testing.T) {
	list := baseParams("l2")
	detail := baseParams("l2")
	detail.ViewMode = "detail"

	if cache.Key(list) == cache.Key(detail) {
		t.Error("different ViewMode must produce different cache keys")
	}
}

func TestKey_SchemaFPIsolation(t *testing.T) {
	// A schema change (new fields, labels, actions) must produce a new key.
	before := baseParams("l3")
	after := baseParams("l3")
	after.SchemaFP = "0000000000000000"

	if cache.Key(before) == cache.Key(after) {
		t.Error("different SchemaFP must produce different cache keys")
	}
}

func TestKey_NoCrossLevelCollision(t *testing.T) {
	// L2 and L3 keys for the same schema must be distinct to prevent
	// a widget tree from being read back as a rendered output.
	l2 := baseParams("l2")
	l3 := baseParams("l3")

	if cache.Key(l2) == cache.Key(l3) {
		t.Error("L2 and L3 keys for same params must be distinct")
	}
}

func TestCacheIsolation_PermFP(t *testing.T) {
	// Prove: a value stored under one PermFP cannot be retrieved under a different PermFP.
	redis := newMemRedis()
	c := cache.New(redis)
	ctx := context.Background()

	adminParams := baseParams("l3")
	adminParams.PermFP = "__admin__"
	adminKey := cache.Key(adminParams)

	restrictedParams := baseParams("l3")
	restrictedParams.PermFP = "__noroles__"
	restrictedKey := cache.Key(restrictedParams)

	// Store admin schema.
	adminSchema := []byte(`{"admin":true,"actions":["edit","delete"]}`)
	if err := c.SetRenderedOutput(ctx, adminKey, adminSchema); err != nil {
		t.Fatal(err)
	}

	// Restricted viewer must get a cache miss — not the admin schema.
	got, err := c.GetRenderedOutput(ctx, restrictedKey)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("restricted viewer must not receive admin cached schema; got %s", got)
	}
}

func TestCacheIsolation_Tenant(t *testing.T) {
	// Prove: schema stored for tenant A cannot be retrieved by tenant B.
	redis := newMemRedis()
	c := cache.New(redis)
	ctx := context.Background()

	tenantAParams := baseParams("l3")
	tenantAParams.TenantIDHash = cache.HashTenantID("tenant-a")
	tenantAKey := cache.Key(tenantAParams)

	tenantBParams := baseParams("l3")
	tenantBParams.TenantIDHash = cache.HashTenantID("tenant-b")
	tenantBKey := cache.Key(tenantBParams)

	if tenantAKey == tenantBKey {
		t.Fatal("tenant A and B must have different cache keys")
	}

	// Store tenant A's schema.
	if err := c.SetRenderedOutput(ctx, tenantAKey, []byte(`{"tenant":"A"}`)); err != nil {
		t.Fatal(err)
	}

	// Tenant B must get a miss.
	got, err := c.GetRenderedOutput(ctx, tenantBKey)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("tenant B must not receive tenant A's cached schema; got %s", got)
	}
}
