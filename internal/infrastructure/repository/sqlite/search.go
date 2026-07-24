package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/search"
)

// SearchRepo implements search.Repository using SQLite.
type SearchRepo struct {
	db *sql.DB
}

// NewSearchRepo creates a new SearchRepo instance.
func NewSearchRepo(db *sql.DB) search.Repository {
	return &SearchRepo{db: db}
}

// SearchByName filters active collections+requests by case-insensitive substring in Go
// (Unicode-correct, unlike SQLite NOCASE). At most filter.Limit hits; LimitReached flags more.
func (r *SearchRepo) SearchByName(ctx context.Context, filter search.Filter) (entities.SearchResult, error) {
	const funcName = "SearchRepo.SearchByName"

	query := `
SELECT id, 'collection' AS kind, name, parent_id AS parent, NULL AS protocol, NULL AS method
  FROM collections
 WHERE workspace_id = ? AND is_delete = 0
UNION ALL
SELECT r.id, 'request' AS kind, r.name, r.collection_id AS parent, r.protocol, r.method
  FROM requests r
  JOIN collections c ON c.id = r.collection_id
 WHERE c.workspace_id = ? AND c.is_delete = 0 AND r.is_delete = 0
 ORDER BY kind, name, id`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query,
		filter.WorkspaceID.String(), filter.WorkspaceID.String(),
	)
	if err != nil {
		return entities.SearchResult{}, fmt.Errorf("%s: query: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	result := entities.SearchResult{Hits: make([]entities.SearchHit, 0)}

	for rows.Next() {
		var (
			idStr     string
			kindStr   string
			name      string
			parentStr sql.NullString
			protoStr  sql.NullString
			methodStr sql.NullString
		)
		if err := rows.Scan(&idStr, &kindStr, &name, &parentStr, &protoStr, &methodStr); err != nil {
			return entities.SearchResult{}, fmt.Errorf("%s: scan: %w", funcName, err)
		}

		if !strings.Contains(strings.ToLower(name), filter.Query) {
			continue
		}

		if len(result.Hits) == filter.Limit {
			result.LimitReached = true
			break
		}

		id, err := uuid.Parse(idStr)
		if err != nil {
			return entities.SearchResult{}, fmt.Errorf("%s: parse id %q: %w", funcName, idStr, err)
		}

		hit := entities.SearchHit{ID: id, Name: name, Kind: entities.SearchHitKind(kindStr)}
		if parentStr.Valid {
			p, err := uuid.Parse(parentStr.String)
			if err != nil {
				return entities.SearchResult{}, fmt.Errorf("%s: parse parent %q: %w", funcName, parentStr.String, err)
			}
			hit.ParentID = &p
		}
		if protoStr.Valid {
			p := entities.Protocol(protoStr.String)
			hit.Protocol = &p
		}
		if methodStr.Valid {
			m := methodStr.String
			hit.Method = &m
		}

		result.Hits = append(result.Hits, hit)
	}

	if err := rows.Err(); err != nil {
		return entities.SearchResult{}, fmt.Errorf("%s: rows: %w", funcName, err)
	}
	return result, nil
}
