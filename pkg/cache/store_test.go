package cache

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	_ Store = (*MemoryStore)(nil)
	_ Store = (*RedisStore)(nil)
)

func TestMemoryStoreContract(t *testing.T) {
	store := NewMemoryStore()
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close memory store: %v", err)
		}
	})
	ctx := context.Background()

	if value, found, err := store.Get(ctx, "missing"); err != nil || found || value != nil {
		t.Fatalf("missing get: value=%v found=%t err=%v", value, found, err)
	}
	if ttl, found, err := store.TTL(ctx, "missing"); err != nil || found || ttl != 0 {
		t.Fatalf("missing ttl: ttl=%v found=%t err=%v", ttl, found, err)
	}

	original := []byte("permanent")
	if err := store.Set(ctx, "permanent", original, 0); err != nil {
		t.Fatalf("set permanent: %v", err)
	}
	original[0] = 'X'
	value, found, err := store.Get(ctx, "permanent")
	if err != nil || !found || string(value) != "permanent" {
		t.Fatalf("get permanent: value=%q found=%t err=%v", value, found, err)
	}
	value[0] = 'Y'
	value, found, err = store.Get(ctx, "permanent")
	if err != nil || !found || string(value) != "permanent" {
		t.Fatalf("get must return a copy: value=%q found=%t err=%v", value, found, err)
	}
	if ttl, found, err := store.TTL(ctx, "permanent"); err != nil || !found || ttl != 0 {
		t.Fatalf("permanent ttl: ttl=%v found=%t err=%v", ttl, found, err)
	}

	if err := store.Set(ctx, "expiring", []byte("value"), 40*time.Millisecond); err != nil {
		t.Fatalf("set expiring: %v", err)
	}
	if ttl, found, err := store.TTL(ctx, "expiring"); err != nil || !found || ttl <= 0 || ttl > 40*time.Millisecond {
		t.Fatalf("expiring ttl: ttl=%v found=%t err=%v", ttl, found, err)
	}
	time.Sleep(60 * time.Millisecond)
	if value, found, err := store.Get(ctx, "expiring"); err != nil || found || value != nil {
		t.Fatalf("expired get: value=%v found=%t err=%v", value, found, err)
	}

	if added, err := store.Add(ctx, "add", []byte("first"), 0); err != nil || !added {
		t.Fatalf("add new: added=%t err=%v", added, err)
	}
	if added, err := store.Add(ctx, "add", []byte("second"), 0); err != nil || added {
		t.Fatalf("add existing: added=%t err=%v", added, err)
	}
	if value, found, err := store.Get(ctx, "add"); err != nil || !found || string(value) != "first" {
		t.Fatalf("add must preserve first value: value=%q found=%t err=%v", value, found, err)
	}
	if err := store.Delete(ctx, "add"); err != nil {
		t.Fatalf("delete existing: %v", err)
	}
	if err := store.Delete(ctx, "add"); err != nil {
		t.Fatalf("delete missing must be idempotent: %v", err)
	}
}

func TestMemoryStoreHonorsCanceledContext(t *testing.T) {
	store := NewMemoryStore()
	defer func() { _ = store.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := store.Get(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("get error = %v, expected context canceled", err)
	}
	if err := store.Set(ctx, "key", []byte("value"), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("set error = %v, expected context canceled", err)
	}
	if _, err := store.Add(ctx, "key", []byte("value"), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("add error = %v, expected context canceled", err)
	}
	if err := store.Delete(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("delete error = %v, expected context canceled", err)
	}
	if _, _, err := store.TTL(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ttl error = %v, expected context canceled", err)
	}
}

func TestMemoryStoreCloseIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestManagerGetAndCloseLifecycle(t *testing.T) {
	manager := NewManager()
	if _, err := manager.Get("missing"); err == nil {
		t.Fatal("expected missing store error")
	}

	memory := NewMemoryStore()
	closer := &recordingStore{}
	manager.Register(" Memory ", memory)
	manager.Register("closer", closer)
	manager.SetDefault(" MEMORY ")
	if store, err := manager.Get(""); err != nil || store != memory {
		t.Fatalf("get default: store=%v err=%v", store, err)
	}
	names := manager.Stores()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("store names must be sorted: %#v", names)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("close manager: %v", err)
	}
	if closer.closeCount.Load() != 1 {
		t.Fatalf("closer count = %d, expected 1", closer.closeCount.Load())
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second manager close: %v", err)
	}
}

func TestRememberJSONUsesSingleflightAndStableType(t *testing.T) {
	store := NewMemoryStore()
	defer func() { _ = store.Close() }()
	type value struct {
		Name string `json:"name"`
	}

	const workers = 16
	start := make(chan struct{})
	var calls atomic.Int64
	results := make(chan value, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := RememberJSON(context.Background(), store, "singleflight", time.Minute, func(context.Context) (value, error) {
				calls.Add(1)
				time.Sleep(20 * time.Millisecond)
				return value{Name: "grove"}, nil
			})
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("remember error: %v", err)
		}
	}
	for result := range results {
		if result.Name != "grove" {
			t.Fatalf("unexpected remembered value: %#v", result)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("loader calls = %d, expected 1", calls.Load())
	}

	result, found, err := GetJSON[value](context.Background(), store, "singleflight")
	if err != nil || !found || result.Name != "grove" {
		t.Fatalf("get json: result=%#v found=%t err=%v", result, found, err)
	}
}

func TestGetJSONReturnsDecodeErrorForInvalidJSON(t *testing.T) {
	store := NewMemoryStore()
	defer func() { _ = store.Close() }()
	if err := store.Set(context.Background(), "invalid-json", []byte("{"), time.Minute); err != nil {
		t.Fatalf("set invalid JSON: %v", err)
	}

	if _, found, err := GetJSON[map[string]string](context.Background(), store, "invalid-json"); err == nil || found {
		t.Fatalf("get invalid JSON: found=%t err=%v", found, err)
	}
}

func TestRememberJSONPropagatesLoaderAndStoreErrors(t *testing.T) {
	loaderErr := errors.New("loader failed")
	store := NewMemoryStore()
	defer func() { _ = store.Close() }()
	if _, err := RememberJSON(context.Background(), store, "loader-error", time.Minute, func(context.Context) (string, error) {
		return "", loaderErr
	}); !errors.Is(err, loaderErr) {
		t.Fatalf("loader error = %v", err)
	}
	if _, found, err := store.Get(context.Background(), "loader-error"); err != nil || found {
		t.Fatalf("loader failure must not be cached: found=%t err=%v", found, err)
	}

	writeErr := errors.New("write failed")
	failing := &recordingStore{setErr: writeErr}
	if _, err := RememberJSON(context.Background(), failing, "write-error", time.Minute, func(context.Context) (string, error) {
		return "value", nil
	}); !errors.Is(err, writeErr) {
		t.Fatalf("write error = %v", err)
	}
}

func TestRememberJSONWaiterCanCancel(t *testing.T) {
	store := NewMemoryStore()
	defer func() { _ = store.Close() }()
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		_, err := RememberJSON(context.Background(), store, "cancel-waiter", time.Minute, func(context.Context) (string, error) {
			close(started)
			<-release
			return "value", nil
		})
		firstDone <- err
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RememberJSON(ctx, store, "cancel-waiter", time.Minute, func(context.Context) (string, error) {
		return "unused", nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter error = %v, expected context canceled", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first remember failed: %v", err)
	}
}

type recordingStore struct {
	mu         sync.Mutex
	data       map[string][]byte
	setErr     error
	closeCount atomic.Int64
}

func (s *recordingStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, found := s.data[key]
	return append([]byte(nil), value...), found, nil
}

func (s *recordingStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string][]byte)
	}
	s.data[key] = append([]byte(nil), value...)
	return nil
}

func (s *recordingStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *recordingStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if _, found, err := s.Get(ctx, key); err != nil || found {
		return false, err
	}
	return true, s.Set(ctx, key, value, ttl)
}

func (s *recordingStore) TTL(_ context.Context, key string) (time.Duration, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, found := s.data[key]
	return 0, found, nil
}

func (s *recordingStore) Close() error {
	s.closeCount.Add(1)
	return nil
}
