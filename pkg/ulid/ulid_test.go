package ulid

import (
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var ulidPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func TestNew_Length(t *testing.T) {
	id := New()
	if len(id) != 26 {
		t.Errorf("length: got %d want 26 (id=%q)", len(id), id)
	}
}

func TestNew_ValidEncoding(t *testing.T) {
	id := New()
	if !ulidPattern.MatchString(id) {
		t.Errorf("id %q does not match Crockford base32 ULID pattern", id)
	}
	for _, r := range id {
		// Crockford excludes I, L, O, U
		if r == 'I' || r == 'L' || r == 'O' || r == 'U' {
			t.Errorf("id %q contains excluded char %q", id, r)
		}
	}
}

func TestNew_Unique(t *testing.T) {
	const n = 10000
	seen := make(map[string]struct{}, n)
	for i := range n {
		id := New()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id after %d iterations: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestNew_TimeSortable(t *testing.T) {
	const n = 100
	var prev string
	for i := range n {
		id := New()
		if i > 0 && strings.Compare(prev, id) > 0 {
			t.Errorf("id went backwards: prev=%q cur=%q", prev, id)
		}
		prev = id
		time.Sleep(time.Millisecond)
	}
}

func TestNew_ConcurrentSafe(t *testing.T) {
	const (
		workers = 50
		per     = 200
	)
	var (
		mu   sync.Mutex
		seen = make(map[string]struct{}, workers*per)
	)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			local := make([]string, per)
			for i := range per {
				local[i] = New()
			}
			mu.Lock()
			for _, id := range local {
				if _, dup := seen[id]; dup {
					t.Errorf("duplicate id under concurrency: %q", id)
				}
				seen[id] = struct{}{}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(seen) != workers*per {
		t.Errorf("expected %d unique ids, got %d", workers*per, len(seen))
	}
}
