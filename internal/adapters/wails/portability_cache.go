package wails

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	previewTTL        = 10 * time.Minute
	previewMaxEntries = 4
	previewMaxBytes   = 8 << 20
)

// previewCache holds a downloaded snapshot between LinkFetch and ImportConfirm: the download counts as
// an import and burns a one-time token, so it must not repeat. Memory only, never the disk.
type previewCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries []previewEntry
}

type previewEntry struct {
	id      string
	data    []byte
	expires time.Time
}

func newPreviewCache() *previewCache {
	return &previewCache{now: time.Now}
}

func (c *previewCache) put(data []byte) (string, error) {
	if len(data) > previewMaxBytes {
		return "", fmt.Errorf("previewCache.put: %d bytes is over the %d byte limit", len(data), previewMaxBytes)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune()
	if len(c.entries) >= previewMaxEntries {
		c.entries = slices.Delete(c.entries, 0, len(c.entries)-previewMaxEntries+1)
	}
	id := uuid.NewString()
	c.entries = append(c.entries, previewEntry{id: id, data: data, expires: c.now().Add(previewTTL)})
	return id, nil
}

func (c *previewCache) get(id string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune()
	for _, e := range c.entries {
		if e.id == id {
			return e.data, true
		}
	}
	return nil, false
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
