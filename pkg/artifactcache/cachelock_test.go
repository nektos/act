package artifactcache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheLock_SameIDIsSerialized(t *testing.T) {
	c := newCacheLock()

	unlock := c.Lock(1)

	acquired := make(chan struct{})
	go func() {
		unlock := c.Lock(1)
		defer unlock()
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("second Lock(1) returned before the first was released")
	case <-time.After(50 * time.Millisecond):
	}

	unlock()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second Lock(1) did not return after the first was released")
	}
}

func TestCacheLock_DifferentIDsDoNotBlock(t *testing.T) {
	c := newCacheLock()

	unlock1 := c.Lock(1)
	defer unlock1()

	done := make(chan struct{})
	go func() {
		unlock2 := c.Lock(2)
		defer unlock2()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Lock(2) blocked on an unrelated Lock(1)")
	}
}

func TestCacheLock_ReleasesMapEntryWhenUncontended(t *testing.T) {
	c := newCacheLock()

	for i := 0; i < 100; i++ {
		unlock := c.Lock(42)
		unlock()
	}

	c.mu.Lock()
	n := len(c.locks)
	c.mu.Unlock()
	assert.Equal(t, 0, n, "cacheLock leaked map entries for a single uncontended ID")
}

func TestCacheLock_ConcurrentUseAcrossManyIDs(t *testing.T) {
	c := newCacheLock()

	var wg sync.WaitGroup
	const goroutines = 50
	const idsPerGoroutine = 20
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < idsPerGoroutine; i++ {
				id := uint64(g%5) * 1000 // deliberately overlap across goroutines
				unlock := c.Lock(id)
				unlock()
			}
		}(g)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent Lock/unlock across overlapping IDs did not complete, possible deadlock")
	}

	c.mu.Lock()
	n := len(c.locks)
	c.mu.Unlock()
	require.Equal(t, 0, n, "cacheLock leaked map entries after all locks were released")
}
