package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, bool, error)
}

type storeCloser interface {
	Close() error
}

type Manager struct {
	mu           sync.RWMutex
	stores       map[string]Store
	defaultStore string
	closeOnce    sync.Once
	closeErr     error
}

func NewManager() *Manager {
	return &Manager{
		stores:       make(map[string]Store),
		defaultStore: "default",
	}
}

func (m *Manager) Register(name string, store Store) {
	if m == nil || store == nil {
		return
	}
	name = normalizeStoreName(name)
	if name == "" {
		return
	}
	m.mu.Lock()
	m.stores[name] = store
	m.mu.Unlock()
}

func (m *Manager) Store(name string) Store {
	store, _ := m.Get(name)
	return store
}

func (m *Manager) Get(name string) (Store, error) {
	if m == nil {
		return nil, fmt.Errorf("cache manager is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	name = normalizeStoreName(name)
	if name == "" {
		name = m.defaultStore
	}
	store, ok := m.stores[name]
	if !ok || store == nil {
		return nil, fmt.Errorf("cache store %q is not configured", name)
	}
	return store, nil
}

func (m *Manager) MustStore(name string) Store {
	store, err := m.Get(name)
	if err != nil {
		panic(err)
	}
	return store
}

func (m *Manager) Default() Store {
	return m.Store("")
}

func (m *Manager) SetDefault(name string) {
	if m == nil {
		return
	}
	name = normalizeStoreName(name)
	if name == "" {
		return
	}
	m.mu.Lock()
	m.defaultStore = name
	m.mu.Unlock()
}

func (m *Manager) Stores() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	names := make([]string, 0, len(m.stores))
	for name := range m.stores {
		names = append(names, name)
	}
	m.mu.RUnlock()
	sort.Strings(names)
	return names
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		m.mu.RLock()
		stores := make([]Store, 0, len(m.stores))
		for _, store := range m.stores {
			stores = append(stores, store)
		}
		m.mu.RUnlock()

		var closeErrs []error
		for _, store := range stores {
			closer, ok := store.(storeCloser)
			if !ok {
				continue
			}
			if err := closer.Close(); err != nil {
				closeErrs = append(closeErrs, err)
			}
		}
		m.closeErr = errors.Join(closeErrs...)
	})
	return m.closeErr
}

func normalizeStoreName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
