package entities

import "github.com/google/uuid"

type SearchHitKind string

const (
	HitCollection SearchHitKind = "collection"
	HitRequest    SearchHitKind = "request"
)

type SearchHit struct {
	ID       uuid.UUID
	Kind     SearchHitKind
	Name     string
	ParentID *uuid.UUID
	Protocol *Protocol
	Method   *string
}

type SearchResult struct {
	Hits         []SearchHit
	LimitReached bool
}
