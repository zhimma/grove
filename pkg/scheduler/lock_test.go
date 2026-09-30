package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/zhimma/grove/pkg/cache"
)

// 在释放操作读到旧值后模拟租期结束和新实例接管。
type changingLockOwnerStore struct {
	*cache.MemoryStore
}

func (s *changingLockOwnerStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, found, err := s.MemoryStore.Get(ctx, key)
	if err != nil || !found {
		return value, found, err
	}
	if err := s.Set(ctx, key, []byte("new-owner"), time.Minute); err != nil {
		return nil, false, err
	}
	return value, found, nil
}

func (s *changingLockOwnerStore) CompareAndDelete(ctx context.Context, key string, expected []byte) (bool, error) {
	if err := s.Set(ctx, key, []byte("new-owner"), time.Minute); err != nil {
		return false, err
	}
	return s.MemoryStore.CompareAndDelete(ctx, key, expected)
}

func TestClusterLockReleasePreservesNewOwner(t *testing.T) {
	store := &changingLockOwnerStore{MemoryStore: cache.NewMemoryStore()}
	t.Cleanup(func() { _ = store.Close() })
	s, err := New(Config{Lock: store})
	if err != nil {
		t.Fatal(err)
	}
	release, acquired, err := s.acquireClusterLock("report")
	if err != nil || !acquired {
		t.Fatalf("acquire: acquired=%v err=%v", acquired, err)
	}
	release()
	value, found, err := store.MemoryStore.Get(context.Background(), clusterLockKey("report"))
	if err != nil || !found || string(value) != "new-owner" {
		t.Fatalf("new owner's lock was removed: value=%q found=%v err=%v", value, found, err)
	}
}
