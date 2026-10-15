package wails

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
)

const previewTTL = 10 * time.Minute

// A download counts as an import and burns a one-time token, so it must not repeat.
type previewCache struct {
	mu         sync.Mutex
	now        func() time.Time
	afterFunc  func(time.Duration, func())
	maxEntries int
	maxBytes   int
	maxTotal   int
	entries    []previewEntry
}

type previewEntry struct {
	id       string
	data     []byte
	fileName string
	expires  time.Time
}

func newPreviewCache() *previewCache {
	return &previewCache{
		now:        time.Now,
		afterFunc:  func(d time.Duration, f func()) { time.AfterFunc(d, f) },
		maxEntries: 4,
		maxBytes:   portability.MaxFileBytes,
		maxTotal:   64 << 20,
	}
}

func (c *previewCache) put(data []byte, fileName string) (string, error) {
	if len(data) > c.maxBytes {
		return "", fmt.Errorf("previewCache.put: %d bytes is over the %d byte limit", len(data), c.maxBytes)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune()
	total := 0
	for _, e := range c.entries {
		total += len(e.data)
	}
	for len(c.entries) > 0 && (len(c.entries) >= c.maxEntries || total+len(data) > c.maxTotal) {
		total -= len(c.entries[0].data)
		c.entries = slices.Delete(c.entries, 0, 1)
	}
	id := uuid.NewString()
	c.entries = append(c.entries, previewEntry{id: id, data: data, fileName: fileName, expires: c.now().Add(previewTTL)})
	// A cancelled preview is never read again, so only a timer frees its bytes.
	c.afterFunc(previewTTL, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.prune()
	})
	return id, nil
}

func (c *previewCache) get(id string) (previewEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune()
	for _, e := range c.entries {
		if e.id == id {
			return e, true
		}
	}
	return previewEntry{}, false
}

func (c *previewCache) remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = slices.DeleteFunc(c.entries, func(e previewEntry) bool { return e.id == id })
}

func (c *previewCache) prune() {
	now := c.now()
	c.entries = slices.DeleteFunc(c.entries, func(e previewEntry) bool { return !now.Before(e.expires) })
}
