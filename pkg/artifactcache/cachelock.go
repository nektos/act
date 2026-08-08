package artifactcache

import "sync"

// cacheLock hands out one mutex per cache ID, so callers operating on
// different cache entries never block each other while callers operating on
// the same entry are fully serialized.
//
// This exists to close the race between the upload and commit handlers
// described in https://github.com/nektos/act/issues/6012: both close their
// database handle before touching storage, so nothing previously prevented
// a commit from finalizing (and, on cleanup, deleting) a cache's temporary
// chunk directory while a concurrent upload was still writing into it.
type cacheLock struct {
	mu    sync.Mutex
	locks map[uint64]*refCountedMutex
}

type refCountedMutex struct {
	mu  sync.Mutex
	ref int
}

func newCacheLock() *cacheLock {
	return &cacheLock{locks: make(map[uint64]*refCountedMutex)}
}

// Lock blocks until the lock for id is held and returns a function that
// releases it. The caller must invoke the returned function exactly once,
// typically via defer, to avoid leaking the entry.
func (c *cacheLock) Lock(id uint64) func() {
	c.mu.Lock()
	l, ok := c.locks[id]
	if !ok {
		l = &refCountedMutex{}
		c.locks[id] = l
	}
	l.ref++
	c.mu.Unlock()

	l.mu.Lock()

	return func() {
		l.mu.Unlock()

		c.mu.Lock()
		l.ref--
		if l.ref == 0 {
			delete(c.locks, id)
		}
		c.mu.Unlock()
	}
}
