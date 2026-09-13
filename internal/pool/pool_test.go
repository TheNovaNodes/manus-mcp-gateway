package pool_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
