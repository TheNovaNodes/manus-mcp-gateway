package pool_test

import (
	"testing"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/pool"
)

func TestMaskID(t *testing.T) {
	if got := pool.MaskID("user@domain.com"); got != "u***@domain.com" {
		t.Errorf("MaskID user@domain.com failed, got %s", got)
	}
	if got := pool.MaskID("@domain.com"); got != "***@domain.com" {
		t.Errorf("MaskID @domain.com failed, got %s", got)
	}
	if got := pool.MaskID("noemail"); got != "noemail" {
		t.Errorf("MaskID noemail failed, got %s", got)
	}
}

func TestSanitizeMessage(t *testing.T) {
	msg := "failed: sk-1234567890123"
	if got := pool.SanitizeMessage(msg); got == msg {
		t.Errorf("SanitizeMessage failed, got %s", got)
	}
}

func TestNextRefreshFormatted(t *testing.T) {
	k := pool.KeyEntry{NextRefreshTime: 0}
	if k.NextRefreshFormatted() != "N/A" {
		t.Errorf("expected N/A, got %s", k.NextRefreshFormatted())
	}
	k2 := pool.KeyEntry{NextRefreshTime: 1726300000}
	if k2.NextRefreshFormatted() == "N/A" {
		t.Errorf("expected formatted date, got N/A")
	}
}

func TestKeyCountAndGetByID(t *testing.T) {
    p := pool.NewPool(nil, []*pool.KeyEntry{
        {ID: "k1"}, {ID: "k2"},
    }, 1*time.Minute)
    
    if p.KeyCount() != 2 {
        t.Errorf("expected 2 keys, got %d", p.KeyCount())
    }
    
    key, err := p.GetKeyByID("k1")
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if key.ID != "k1" {
        t.Errorf("expected k1, got %s", key.ID)
    }
    
    _, err = p.GetKeyByID("k3")
    if err == nil {
        t.Errorf("expected error for non-existent key")
    }
}
