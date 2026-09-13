package pool

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/manus"
)

var (
	// ErrNoKeysInPool indicates the pool has 0 configured keys.
	ErrNoKeysInPool = errors.New("no Manus API keys configured in pool")

	// ErrNoAvailableKeys indicates all keys are rate limited, exhausted, or errored.
	ErrNoAvailableKeys = errors.New("all keys in capacity pool are currently exhausted or rate limited")
)

var keyRegex = regexp.MustCompile(`sk-[a-zA-Z0-9_-]+`)

// ParseKeys parses raw string of keys into slice of KeyEntry.
// Supports:
// 1. "email1@domain.com sk-... email2@domain.com sk-..."
// 2. "sk-key1,sk-key2,sk-key3"
// 3. "sk-key1\nsk-key2"
func ParseKeys(raw string) []*KeyEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Replace commas, semicolons, tabs, and newlines with spaces
	normalized := strings.Map(func(r rune) rune {
		if r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, raw)

	var entries []*KeyEntry
	tokens := strings.Fields(normalized)

	// Attempt token-based pairing (<identifier> <sk-key>)
	for i := 0; i < len(tokens); i++ {
		match := keyRegex.FindString(tokens[i])
		if match != "" {
			id := fmt.Sprintf("Key_%d", len(entries)+1)
			if i > 0 && keyRegex.FindString(tokens[i-1]) == "" {
				candidateID := strings.Trim(tokens[i-1], "\"'=:")
				if idx := strings.Index(candidateID, "="); idx != -1 {
					candidateID = candidateID[idx+1:]
				}
				candidateID = strings.Trim(candidateID, "\"'=:")
				if candidateID != "" && candidateID != "MANUS_KEYS" {
					id = candidateID
				}
			}
			key := match
			entries = append(entries, &KeyEntry{
				ID:        id,
				Key:       key,
				MaskedKey: MaskAPIKey(key),
				Status:    StatusHealthy,
			})
		}
	}

	// Fallback to pure regex if no pairs were matched
	if len(entries) == 0 {
		matches := keyRegex.FindAllString(raw, -1)
		for i, k := range matches {
			entries = append(entries, &KeyEntry{
				ID:        fmt.Sprintf("Key_%d", i+1),
				Key:       k,
				MaskedKey: MaskAPIKey(k),
				Status:    StatusHealthy,
			})
		}
	}

	return entries
}

// Pool coordinates multi-key state, greedy credit balancing, and failover.
type Pool struct {
	mu          sync.RWMutex
	client      *manus.Client
	keys        []*KeyEntry
	cacheTTL    time.Duration
	lastChecked time.Time
	rrIndex     int
	taskCache   sync.Map
}

// NewPool initializes a new key pool.
func NewPool(client *manus.Client, keys []*KeyEntry, cacheTTL time.Duration) *Pool {
	if cacheTTL <= 0 {
		cacheTTL = 3 * time.Minute
	}
	return &Pool{
		client:   client,
		keys:     keys,
		cacheTTL: cacheTTL,
	}
}

// KeyCount returns total keys in pool.
func (p *Pool) KeyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.keys)
}

// RefreshBalances queries /v2/usage.availableCredits for all keys concurrently.
func (p *Pool) RefreshBalances(ctx context.Context, force bool) error {
	p.mu.Lock()
	if !force && time.Since(p.lastChecked) < p.cacheTTL && p.lastChecked != (time.Time{}) {
		p.mu.Unlock()
		return nil
	}
	keys := make([]*KeyEntry, len(p.keys))
	copy(keys, p.keys)
	p.mu.Unlock()

	if len(keys) == 0 {
		return ErrNoKeysInPool
	}

	var wg sync.WaitGroup
	now := time.Now()

	for _, k := range keys {
		wg.Add(1)
		go func(entry *KeyEntry) {
			defer wg.Done()
			reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()

			credits, err := p.client.GetAvailableCredits(reqCtx, entry.Key)
			p.mu.Lock()
			defer p.mu.Unlock()

			entry.LastChecked = now
			if err != nil {
				entry.LastError = err.Error()
				if errors.Is(err, manus.ErrRateLimited) {
					entry.Status = StatusRateLimited
					backoffDuration := 5 * time.Minute
					var rateLimitErr *manus.RateLimitError
					if errors.As(err, &rateLimitErr) && rateLimitErr.RetryAfter > 0 {
						backoffDuration = rateLimitErr.RetryAfter
					}
					entry.BackoffUntil = now.Add(backoffDuration)
				} else if errors.Is(err, manus.ErrUnauthorized) {
					entry.Status = StatusError
				} else {
					entry.Status = StatusError
				}
				return
			}

			entry.TotalCredits = credits.TotalCredits
			entry.RefreshCredits = credits.RefreshCredits
			entry.MaxRefresh = credits.MaxRefreshCredits
			entry.NextRefreshTime = credits.NextRefreshTime.Int64()
			entry.RefreshInterval = credits.RefreshInterval
			entry.LastError = ""

			if credits.TotalCredits == 0 {
				entry.Status = StatusExhausted
			} else if time.Now().Before(entry.BackoffUntil) {
				entry.Status = StatusRateLimited
			} else {
				entry.Status = StatusHealthy
			}
		}(k)
	}

	wg.Wait()

	p.mu.Lock()
	p.lastChecked = now
	p.mu.Unlock()
	return nil
}

// GetPoolStatus returns full pool summary.
func (p *Pool) GetPoolStatus(ctx context.Context) (*PoolStatus, error) {
	if err := p.RefreshBalances(ctx, false); err != nil && !errors.Is(err, ErrNoKeysInPool) {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	totalCredits := 0
	activeCount := 0
	keysCopy := make([]*KeyEntry, len(p.keys))

	for i, k := range p.keys {
		copyEntry := *k
		keysCopy[i] = &copyEntry

		totalCredits += k.TotalCredits
		if k.Status == StatusHealthy && (k.BackoffUntil.IsZero() || now.After(k.BackoffUntil)) {
			activeCount++
		}
	}

	return &PoolStatus{
		TotalKeys:        len(p.keys),
		ActiveKeys:       activeCount,
		TotalCreditsPool: totalCredits,
		Keys:             keysCopy,
		CheckedAt:        p.lastChecked,
	}, nil
}

// PickKey selects best key using Greedy Credit Routing (key with highest credits).
// If ties exist, uses round-robin.
func (p *Pool) PickKey(ctx context.Context) (*KeyEntry, error) {
	if err := p.RefreshBalances(ctx, false); err != nil && !errors.Is(err, ErrNoKeysInPool) {
		// Log or continue with cached state
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return nil, ErrNoKeysInPool
	}

	now := time.Now()
	var candidates []*KeyEntry

	for _, k := range p.keys {
		// Ignore keys currently in backoff or error
		if k.Status == StatusError {
			continue
		}
		if !k.BackoffUntil.IsZero() && now.Before(k.BackoffUntil) {
			continue
		}
		if k.TotalCredits <= 0 && k.Status == StatusExhausted {
			continue
		}
		candidates = append(candidates, k)
	}

	// Fallback: if all healthy keys exhausted, pick any non-error key whose backoff has passed
	if len(candidates) == 0 {
		for _, k := range p.keys {
			if k.Status != StatusError {
				candidates = append(candidates, k)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, ErrNoAvailableKeys
	}

	// Greedy selection: pick key with maximum TotalCredits
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.TotalCredits > best.TotalCredits {
			best = c
		}
	}

	// If multiple have the exact same maximum credit balance, round-robin among them
	var topTier []*KeyEntry
	for _, c := range candidates {
		if c.TotalCredits == best.TotalCredits {
			topTier = append(topTier, c)
		}
	}

	if len(topTier) > 1 {
		p.rrIndex = (p.rrIndex + 1) % len(topTier)
		best = topTier[p.rrIndex]
	}

	copyEntry := *best
	return &copyEntry, nil
}

// MarkRateLimited sets backoff on a key by ID.
func (p *Pool) MarkRateLimited(keyID string, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if duration <= 0 {
		duration = 5 * time.Minute
	}
	now := time.Now()
	for _, k := range p.keys {
		if k.ID == keyID {
			k.Status = StatusRateLimited
			k.BackoffUntil = now.Add(duration)
			break
		}
	}
}

// GetKeyByID finds key entry by ID.
func (p *Pool) GetKeyByID(keyID string) (*KeyEntry, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, k := range p.keys {
		if k.ID == keyID {
			copyEntry := *k
			return &copyEntry, nil
		}
	}
	return nil, fmt.Errorf("key with ID '%s' not found", keyID)
}

// FindKeyForTask searches across all keys to locate which key created the task.
func (p *Pool) FindKeyForTask(ctx context.Context, taskID string) (*KeyEntry, error) {
	if val, ok := p.taskCache.Load(taskID); ok {
		if cachedEntry, ok := val.(*KeyEntry); ok {
			copyEntry := *cachedEntry
			return &copyEntry, nil
		}
	}

	p.mu.RLock()
	keys := make([]*KeyEntry, len(p.keys))
	copy(keys, p.keys)
	p.mu.RUnlock()

	searchCtx, cancelSearch := context.WithCancel(ctx)
	defer cancelSearch()

	var wg sync.WaitGroup
	resultChan := make(chan *KeyEntry, len(keys))

	for _, k := range keys {
		wg.Add(1)
		go func(entry *KeyEntry) {
			defer wg.Done()
			reqCtx, cancelReq := context.WithTimeout(searchCtx, 4*time.Second)
			defer cancelReq()

			detail, err := p.client.GetTaskDetail(reqCtx, entry.Key, taskID)
			if err == nil && detail != nil && detail.ID == taskID {
				select {
				case resultChan <- entry:
					cancelSearch() // Stop other goroutines
				case <-searchCtx.Done():
				}
			}
		}(k)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	if foundEntry, ok := <-resultChan; ok {
		p.taskCache.Store(taskID, foundEntry)
		copyEntry := *foundEntry
		return &copyEntry, nil
	}

	return nil, fmt.Errorf("task %s not found on any configured key in pool", taskID)
}

// AssociateTaskKey stores the task-to-key association in the cache.
func (p *Pool) AssociateTaskKey(taskID string, entry *KeyEntry) {
	p.taskCache.Store(taskID, entry)
}
