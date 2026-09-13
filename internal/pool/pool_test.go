package pool_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/manus"
	"github.com/TheNovaNodes/manus-mcp-gateway/internal/pool"
)

func TestParseKeys(t *testing.T) {
	raw := "acc1@novanodes.ai sk-key1 acc2@novanodes.ai sk-key2 acc3@novanodes.ai sk-key3"
	keys := pool.ParseKeys(raw)

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0].ID != "acc1@novanodes.ai" || keys[0].Key != "sk-key1" {
		t.Errorf("unexpected key 0: %+v", keys[0])
	}
	if keys[1].ID != "acc2@novanodes.ai" || keys[1].Key != "sk-key2" {
		t.Errorf("unexpected key 1: %+v", keys[1])
	}
	if keys[0].MaskedKey != "sk-...y1" {
		t.Errorf("unexpected masked key: %s", keys[0].MaskedKey)
	}

	// Comma separated
	rawComma := "sk-1111111111,sk-2222222222"
	keysComma := pool.ParseKeys(rawComma)
	if len(keysComma) != 2 {
		t.Fatalf("expected 2 keys from comma string, got %d", len(keysComma))
	}

	// Environment variable with quotes and prefixes
	rawEnv := "MANUS_KEYS=\"user1@test.com sk-clean-1 user2@test.com sk-dirty-2\""
	keysEnv := pool.ParseKeys(rawEnv)
	if len(keysEnv) != 2 {
		t.Fatalf("expected 2 keys from env string, got %d", len(keysEnv))
	}
	if keysEnv[0].ID != "user1@test.com" || keysEnv[0].Key != "sk-clean-1" {
		t.Errorf("unexpected key 0 from env: %+v", keysEnv[0])
	}
	if keysEnv[1].ID != "user2@test.com" || keysEnv[1].Key != "sk-dirty-2" {
		t.Errorf("unexpected key 1 from env: %+v", keysEnv[1])
	}
}

func TestGreedyCreditSelection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("x-manus-api-key")
		credits := 100
		if key == "sk-rich-key" {
			credits = 300
		} else if key == "sk-poor-key" {
			credits = 25
		}

		resp := manus.AvailableCreditsResponse{
			OK: true,
			Data: &manus.AvailableCredits{
				TotalCredits:      credits,
				RefreshCredits:    credits,
				MaxRefreshCredits: 300,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))

	keys := []*pool.KeyEntry{
		{ID: "PoorKey", Key: "sk-poor-key"},
		{ID: "RichKey", Key: "sk-rich-key"},
		{ID: "MidKey", Key: "sk-mid-key"},
	}

	p := pool.NewPool(client, keys, 1*time.Minute)
	ctx := context.Background()

	// Should pick RichKey because it has 300 credits
	picked, err := p.PickKey(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if picked.ID != "RichKey" {
		t.Errorf("expected RichKey, got %s (credits: %d)", picked.ID, picked.TotalCredits)
	}

	// Now mark RichKey as rate limited
	p.MarkRateLimited("RichKey", 5*time.Minute)

	// Next pick should be MidKey (100 credits > 25 credits)
	picked2, err := p.PickKey(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked2.ID != "MidKey" {
		t.Errorf("expected MidKey after rate limit on RichKey, got %s", picked2.ID)
	}
}

func TestFindKeyForTask_ConcurrentAndCache(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		key := r.Header.Get("x-manus-api-key")
		time.Sleep(50 * time.Millisecond) // simulate latency

		if key == "sk-target-key" && r.URL.Query().Get("task_id") == "task_target" {
			resp := manus.TaskDetailResponse{
				OK: true,
				Data: &manus.TaskDetailData{
					ID: "task_target",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// return 404 for others
		w.WriteHeader(http.StatusNotFound)
		resp := manus.StandardResponse{
			OK:    false,
			Error: &manus.APIError{Code: "not_found", Message: "task not found"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))

	keys := []*pool.KeyEntry{
		{ID: "K1", Key: "sk-key1"},
		{ID: "K2", Key: "sk-key2"},
		{ID: "Target", Key: "sk-target-key"},
		{ID: "K3", Key: "sk-key3"},
		{ID: "K4", Key: "sk-key4"},
	}

	p := pool.NewPool(client, keys, 1*time.Minute)
	ctx := context.Background()

	// Initial lookup - should take ~50ms and return Target
	start := time.Now()
	entry, err := p.FindKeyForTask(ctx, "task_target")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != "Target" {
		t.Errorf("expected Target key, got %s", entry.ID)
	}
	if duration > 100*time.Millisecond { // 50ms sleep + a small buffer. If they ran sequentially it would take 250ms+
		t.Errorf("lookup took too long (%v), likely not concurrent", duration)
	}

	// Secondary lookup - should be instant via cache
	requestCountBefore := atomic.LoadInt32(&requestCount)
	startCached := time.Now()
	cachedEntry, err := p.FindKeyForTask(ctx, "task_target")
	durationCached := time.Since(startCached)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cachedEntry.ID != "Target" {
		t.Errorf("expected Target key, got %s", cachedEntry.ID)
	}
	if atomic.LoadInt32(&requestCount) != requestCountBefore {
		t.Errorf("expected no additional requests, but got %d", atomic.LoadInt32(&requestCount)-requestCountBefore)
	}
	if durationCached > 10*time.Millisecond {
		t.Errorf("cached lookup took too long (%v), cache likely not hit", durationCached)
	}

	// AssociateTaskKey test
	p.AssociateTaskKey("task_injected", keys[0])
	injectedEntry, err := p.FindKeyForTask(ctx, "task_injected")
	if err != nil {
		t.Fatalf("unexpected error for injected task: %v", err)
	}
	if injectedEntry.ID != "K1" {
		t.Errorf("expected injected task to map to K1, got %s", injectedEntry.ID)
	}
}

func TestPoolStatusAggregation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := manus.AvailableCreditsResponse{
			OK: true,
			Data: &manus.AvailableCredits{
				TotalCredits:      300,
				RefreshCredits:    300,
				MaxRefreshCredits: 300,
				NextRefreshTime:   1726300000,
				RefreshInterval:   "daily",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))

	keys := []*pool.KeyEntry{
		{ID: "Key1", Key: "sk-key1"},
		{ID: "Key2", Key: "sk-key2"},
		{ID: "Key3", Key: "sk-key3"},
		{ID: "Key4", Key: "sk-key4"},
		{ID: "Key5", Key: "sk-key5"},
		{ID: "Key6", Key: "sk-key6"},
		{ID: "Key7", Key: "sk-key7"},
	}

	p := pool.NewPool(client, keys, 1*time.Minute)
	status, err := p.GetPoolStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.TotalKeys != 7 {
		t.Errorf("expected 7 total keys, got %d", status.TotalKeys)
	}
	if status.TotalCreditsPool != 2100 {
		t.Errorf("expected 2100 total credits pool (7x300), got %d", status.TotalCreditsPool)
	}
	if status.ActiveKeys != 7 {
		t.Errorf("expected 7 active keys, got %d", status.ActiveKeys)
	}
}
