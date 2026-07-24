package dto

import "github.com/tetiva-app/client/internal/domain/entities"

// SearchHitDTO is the wire format of a single search match.
type SearchHitDTO struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId,omitempty"`
	Protocol *string `json:"protocol,omitempty"`
	Method   *string `json:"method,omitempty"`
}

// SearchResponse is the wire format of search results.
type SearchResponse struct {
	Hits         []SearchHitDTO `json:"hits"`
	LimitReached bool           `json:"limitReached"`
}

// ToSearchResponse converts the domain result into a wire DTO.
func ToSearchResponse(r entities.SearchResult) SearchResponse {
	hits := make([]SearchHitDTO, len(r.Hits))
	for i, h := range r.Hits {
		d := SearchHitDTO{
			ID:   h.ID.String(),
			Kind: string(h.Kind),
			Name: h.Name,
		}
		if h.ParentID != nil {
			s := h.ParentID.String()
			d.ParentID = &s
		}
		if h.Protocol != nil {
			s := string(*h.Protocol)
			d.Protocol = &s
		}
		if h.Method != nil {
			s := *h.Method
			d.Method = &s
		}
		hits[i] = d
	}
	return SearchResponse{Hits: hits, LimitReached: r.LimitReached}
}
