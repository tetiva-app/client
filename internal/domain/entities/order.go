package entities

import (
	"bytes"
	"slices"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// SortCollections puts siblings in sidebar order: name, then creation time, then id.
func SortCollections(list []*Collection) {
	c := nameCollator()
	slices.SortFunc(list, func(a, b *Collection) int {
		return compareSiblings(c, a.Name, a.CreatedAt, a.ID, b.Name, b.CreatedAt, b.ID)
	})
}

// SortRequests puts siblings in sidebar order: name, then creation time, then id.
func SortRequests(list []*Request) {
	c := nameCollator()
	slices.SortFunc(list, func(a, b *Request) int {
		return compareSiblings(c, a.Name, a.CreatedAt, a.ID, b.Name, b.CreatedAt, b.ID)
	})
}

func nameCollator() *collate.Collator {
	return collate.New(language.English, collate.Loose, collate.Numeric)
}

func compareSiblings(c *collate.Collator, aName string, aAt time.Time, aID uuid.UUID, bName string, bAt time.Time, bID uuid.UUID) int {
	if r := c.CompareString(aName, bName); r != 0 {
		return r
	}
	if r := aAt.Compare(bAt); r != 0 {
		return r
	}
	return bytes.Compare(aID[:], bID[:])
}
