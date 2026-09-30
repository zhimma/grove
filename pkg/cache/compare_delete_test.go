package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

type conditionalStore interface {
	Store
	CompareAndDelete(context.Context, string, []byte) (bool, error)
}

func TestMemoryStoreCompareAndDelete(t *testing.T) {
	store := NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })
	testCompareAndDelete(t, store)
}

func testCompareAndDelete(t *testing.T, store conditionalStore) {
	t.Helper()
	ctx := context.Background()
	const key = "compare-delete"
	t.Cleanup(func() { _ = store.Delete(context.Background(), key) })
	if deleted, err := store.CompareAndDelete(ctx, key, []byte("old")); err != nil || deleted {
		t.Fatalf("missing key: deleted=%v err=%v", deleted, err)
	}
	if err := store.Set(ctx, key, []byte("new"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if deleted, err := store.CompareAndDelete(ctx, key, []byte("old")); err != nil || deleted {
		t.Fatalf("different owner: deleted=%v err=%v", deleted, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if deleted, err := store.CompareAndDelete(canceled, key, []byte("new")); deleted || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled delete: deleted=%v err=%v", deleted, err)
	}
	if value, found, err := store.Get(ctx, key); err != nil || !found || string(value) != "new" {
		t.Fatalf("new owner lost its value: value=%q found=%v err=%v", value, found, err)
	}
	if deleted, err := store.CompareAndDelete(ctx, key, []byte("new")); err != nil || !deleted {
		t.Fatalf("matching owner: deleted=%v err=%v", deleted, err)
	}
	if _, found, err := store.Get(ctx, key); err != nil || found {
		t.Fatalf("value survived delete: found=%v err=%v", found, err)
	}
}
