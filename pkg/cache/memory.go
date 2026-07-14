package cache

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	mu        sync.Mutex
	data      map[string]memoryItem
	stopCh    chan struct{}
	doneCh    chan struct{}
	closeOnce sync.Once
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		data:   make(map[string]memoryItem),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go store.gc()
	return store
}

func (m *MemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := contextError(ctx); err != nil {
		return nil, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, found := m.data[key]
	if !found {
		return nil, false, nil
	}
	if item.expired(time.Now()) {
		delete(m.data, key)
		return nil, false, nil
	}
	return append([]byte(nil), item.value...), true, nil
}

func (m *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	item := memoryItem{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.data[key] = item
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) Delete(ctx context.Context, key string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if err := contextError(ctx); err != nil {
		return false, err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if item, found := m.data[key]; found && !item.expired(now) {
		return false, nil
	}
	item := memoryItem{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = now.Add(ttl)
	}
	m.data[key] = item
	return true, nil
}

func (m *MemoryStore) TTL(ctx context.Context, key string) (time.Duration, bool, error) {
	if err := contextError(ctx); err != nil {
		return 0, false, err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	item, found := m.data[key]
	if !found {
		return 0, false, nil
	}
	if item.expired(now) {
		delete(m.data, key)
		return 0, false, nil
	}
	if item.expiresAt.IsZero() {
		return 0, true, nil
	}
	return item.expiresAt.Sub(now), true, nil
}

func (m *MemoryStore) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		close(m.stopCh)
	})
	<-m.doneCh
	return nil
}

func (m *MemoryStore) gc() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	defer close(m.doneCh)
	for {
		select {
		case now := <-ticker.C:
			m.cleanup(now)
		case <-m.stopCh:
			return
		}
	}
}

func (m *MemoryStore) cleanup(now time.Time) {
	m.mu.Lock()
	for key, item := range m.data {
		if item.expired(now) {
			delete(m.data, key)
		}
	}
	m.mu.Unlock()
}

func (item memoryItem) expired(now time.Time) bool {
	return !item.expiresAt.IsZero() && !now.Before(item.expiresAt)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
