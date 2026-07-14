package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type LoginConfig struct {
	AttemptsPerMinute int
	Burst             int
	FailureLimit      int
	LockDuration      time.Duration
}

type LoginGuard interface {
	Allow(ctx context.Context, key string) error
	Locked(ctx context.Context, key string) (time.Duration, error)
	Failure(ctx context.Context, key string) (time.Duration, error)
	Reset(ctx context.Context, key string) error
}

type LimitError struct {
	retryAfter time.Duration
}

func (e *LimitError) Error() string {
	return "login rate limit exceeded"
}

func (e *LimitError) RetryAfter() time.Duration {
	if e == nil {
		return 0
	}
	return e.retryAfter
}

func RetryAfter(err error) time.Duration {
	var limitErr *LimitError
	if errors.As(err, &limitErr) {
		return limitErr.RetryAfter()
	}
	return 0
}

func LoginKey(account, clientIP string) string {
	normalizedAccount := strings.ToLower(strings.TrimSpace(account))
	normalizedIP := strings.ToLower(strings.TrimSpace(clientIP))
	if parsed := net.ParseIP(normalizedIP); parsed != nil {
		normalizedIP = parsed.String()
	}
	sum := sha256.Sum256([]byte(normalizedAccount + "\x00" + normalizedIP))
	return hex.EncodeToString(sum[:])
}

func NewLoginGuard(cfg LoginConfig, client *redis.Client) LoginGuard {
	cfg = normalizeLoginConfig(cfg)
	if client != nil {
		return NewRedisLoginGuard(client, cfg)
	}
	return NewLocalLoginGuard(cfg)
}

type LocalLoginGuard struct {
	mu          sync.Mutex
	cfg         LoginConfig
	entries     map[string]*localLoginEntry
	lastCleanup time.Time
	staleAfter  time.Duration
}

type localLoginEntry struct {
	limiter     *rate.Limiter
	failures    int
	lockedUntil time.Time
	lastSeen    time.Time
}

func NewLocalLoginGuard(cfg LoginConfig) *LocalLoginGuard {
	cfg = normalizeLoginConfig(cfg)
	staleAfter := 10 * time.Minute
	if candidate := cfg.LockDuration * 2; candidate > staleAfter {
		staleAfter = candidate
	}
	return &LocalLoginGuard{
		cfg:        cfg,
		entries:    make(map[string]*localLoginEntry),
		staleAfter: staleAfter,
	}
}

func (g *LocalLoginGuard) Allow(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.entryLocked(strings.TrimSpace(key), now)
	if entry.limiter.AllowN(now, 1) {
		return nil
	}

	tokens := entry.limiter.TokensAt(now)
	seconds := (1 - tokens) / float64(entry.limiter.Limit())
	retryAfter := time.Duration(math.Ceil(seconds * float64(time.Second)))
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return &LimitError{retryAfter: retryAfter}
}

func (g *LocalLoginGuard) Locked(ctx context.Context, key string) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.entryLocked(strings.TrimSpace(key), now)
	return g.lockedDurationLocked(entry, now), nil
}

func (g *LocalLoginGuard) Failure(ctx context.Context, key string) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.entryLocked(strings.TrimSpace(key), now)
	if retryAfter := g.lockedDurationLocked(entry, now); retryAfter > 0 {
		return retryAfter, nil
	}

	entry.failures++
	if entry.failures < g.cfg.FailureLimit {
		return 0, nil
	}
	entry.failures = 0
	entry.lockedUntil = now.Add(g.cfg.LockDuration)
	return g.cfg.LockDuration, nil
}

func (g *LocalLoginGuard) Reset(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.entryLocked(strings.TrimSpace(key), now)
	entry.failures = 0
	entry.lockedUntil = time.Time{}
	return nil
}

func (g *LocalLoginGuard) entryLocked(key string, now time.Time) *localLoginEntry {
	g.cleanupLocked(now)
	entry := g.entries[key]
	if entry == nil {
		limit := rate.Limit(float64(g.cfg.AttemptsPerMinute) / 60)
		entry = &localLoginEntry{limiter: rate.NewLimiter(limit, g.cfg.Burst)}
		g.entries[key] = entry
	}
	entry.lastSeen = now
	return entry
}

func (g *LocalLoginGuard) lockedDurationLocked(entry *localLoginEntry, now time.Time) time.Duration {
	if entry.lockedUntil.IsZero() {
		return 0
	}
	if !now.Before(entry.lockedUntil) {
		entry.failures = 0
		entry.lockedUntil = time.Time{}
		return 0
	}
	return entry.lockedUntil.Sub(now)
}

func (g *LocalLoginGuard) cleanupLocked(now time.Time) {
	if !g.lastCleanup.IsZero() && now.Sub(g.lastCleanup) < time.Minute {
		return
	}
	for key, entry := range g.entries {
		if now.Sub(entry.lastSeen) > g.staleAfter && !now.Before(entry.lockedUntil) {
			delete(g.entries, key)
		}
	}
	g.lastCleanup = now
}

func normalizeLoginConfig(cfg LoginConfig) LoginConfig {
	if cfg.AttemptsPerMinute <= 0 {
		cfg.AttemptsPerMinute = 10
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 5
	}
	if cfg.FailureLimit <= 0 {
		cfg.FailureLimit = 5
	}
	if cfg.LockDuration <= 0 {
		cfg.LockDuration = 15 * time.Minute
	}
	return cfg
}
