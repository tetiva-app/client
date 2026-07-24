package entities

import "github.com/google/uuid"

// SearchHitKind differentiates between collection and request hits.
type SearchHitKind string

const (
	HitCollection SearchHitKind = "collection"
	HitRequest    SearchHitKind = "request"
)

// SearchHit is a single search match (collection or request).
type SearchHit struct {
	ID       uuid.UUID
	Kind     SearchHitKind
	Name     string
	ParentID *uuid.UUID
	Protocol *Protocol
	Method   *string
}

// SearchResult is the outcome of a name search.
type SearchResult struct {
	Hits         []SearchHit
	LimitReached bool
}
