package pool

import (
	"fmt"
	"strings"
	"time"
)

// KeyStatus represents health and availability status of an API key.
type KeyStatus string

const (
	StatusHealthy     KeyStatus = "HEALTHY"
	StatusRateLimited KeyStatus = "RATE_LIMITED"
	StatusExhausted   KeyStatus = "EXHAUSTED" // 0 credits
	StatusError       KeyStatus = "ERROR"
)

// KeyEntry stores metadata, balance, and status of a single Manus API key.
type KeyEntry struct {
	ID              string    `json:"id"`
	Key             string    `json:"-"`
	MaskedKey       string    `json:"masked_key"`
	TotalCredits    int       `json:"total_credits"`
	RefreshCredits  int       `json:"refresh_credits"`
	MaxRefresh      int       `json:"max_refresh"`
	NextRefreshTime int64     `json:"next_refresh_time"`
	RefreshInterval string    `json:"refresh_interval"`
	Status          KeyStatus `json:"status"`
	LastError       string    `json:"last_error,omitempty"`
	BackoffUntil    time.Time `json:"backoff_until,omitempty"`
	LastChecked     time.Time `json:"last_checked"`
}

// NextRefreshFormatted returns formatted UTC timestamp of next refresh.
func (k *KeyEntry) NextRefreshFormatted() string {
	if k.NextRefreshTime <= 0 {
		return "N/A"
	}
	return time.Unix(k.NextRefreshTime, 0).UTC().Format(time.RFC3339)
}

// MaskAPIKey masks sensitive key characters.
func MaskAPIKey(k string) string {
	k = strings.TrimSpace(k)
	if len(k) <= 6 {
		return "***"
	}
	if len(k) <= 10 {
		return fmt.Sprintf("%s...%s", k[:3], k[len(k)-2:])
	}
	return fmt.Sprintf("%s...%s", k[:4], k[len(k)-4:])
}

// PoolStatus summary of the entire 7-key capacity pool.
type PoolStatus struct {
	TotalKeys        int         `json:"total_keys"`
	ActiveKeys       int         `json:"active_keys"`
	TotalCreditsPool int         `json:"total_credits_pool"`
	Keys             []*KeyEntry `json:"keys"`
	CheckedAt        time.Time   `json:"checked_at"`
}
