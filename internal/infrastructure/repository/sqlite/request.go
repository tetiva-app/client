package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type headerItemJSON struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type RequestRepo struct {
	db *sql.DB
}

func NewRequestRepo(db *sql.DB) request.Repository {
	return &RequestRepo{db: db}
}

func (r *RequestRepo) Create(ctx context.Context, req *entities.Request) error {
	const funcName = "RequestRepo.Create"

	headersJSON, err := json.Marshal(headersToJSON(req.Headers))
	if err != nil {
		return fmt.Errorf("%s: marshal headers: %w", funcName, err)
	}

	metadataJSON, err := json.Marshal(req.GRPCMetadata)
	if err != nil {
		return fmt.Errorf("%s: marshal grpc_metadata: %w", funcName, err)
	}

	query := `INSERT INTO requests (id, collection_id, name, description, protocol, method, url, headers, body, body_type,
		auth_type, auth_data,
		grpc_service, grpc_method, grpc_proto_path, grpc_metadata,
		graphql_query, graphql_variables, graphql_schema_path, graphql_operation,
		pre_script, post_script, sort_order, version, is_delete, is_draft, created_by, created_at, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		req.ID.String(),
		req.CollectionID.String(),
		req.Name,
		req.Description,
		string(req.Protocol),
		string(req.Method),
		req.URL,
		string(headersJSON),
		req.Body,
		string(req.BodyType),
		string(req.AuthType),
		req.AuthData,
		req.GRPCService,
		req.GRPCMethod,
		req.GRPCProtoPath,
		string(metadataJSON),
		req.GraphQLQuery,
		req.GraphQLVariables,
		req.GraphQLSchemaPath,
		req.GraphQLOperation,
		req.PreScript,
		req.PostScript,
		req.SortOrder,
		req.Version,
		boolToInt(req.IsDelete),
		boolToInt(req.IsDraft),
		req.CreatedBy,
		req.CreatedAt.Format(time.RFC3339),
		req.UpdatedBy,
		req.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Returns nil when the row is missing or soft-deleted.
func (r *RequestRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error) {
	const funcName = "RequestRepo.GetByID"

	query := `SELECT id, collection_id, name, description, protocol, method, url, headers, body, body_type,
		auth_type, auth_data,
		grpc_service, grpc_method, grpc_proto_path, grpc_metadata,
		graphql_query, graphql_variables, graphql_schema_path, graphql_operation,
		pre_script, post_script, sort_order, version, is_delete, is_draft, created_by, created_at, updated_by, updated_at
		FROM requests WHERE id = ? AND is_delete = 0`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, id.String())

	req, err := scanRequest(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return req, nil
}

// Ordered by sort_order ASC, created_at ASC.
func (r *RequestRepo) List(ctx context.Context, filter request.Filter) ([]*entities.Request, error) {
	const funcName = "RequestRepo.List"

	query := `SELECT id, collection_id, name, description, protocol, method, url, headers, body, body_type,
		auth_type, auth_data,
		grpc_service, grpc_method, grpc_proto_path, grpc_metadata,
		graphql_query, graphql_variables, graphql_schema_path, graphql_operation,
		pre_script, post_script, sort_order, version, is_delete, is_draft, created_by, created_at, updated_by, updated_at
		FROM requests WHERE collection_id = ? AND is_delete = 0 AND is_draft = 0
		ORDER BY sort_order ASC, created_at ASC`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query, filter.CollectionID.String())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.Request
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", funcName, err)
		}
		result = append(result, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", funcName, err)
	}

	return result, nil
}

// No version guard — sync applies server changes; the UI does optimistic locking one layer up.
func (r *RequestRepo) Update(ctx context.Context, req *entities.Request) error {
	const funcName = "RequestRepo.Update"

	headersJSON, err := json.Marshal(headersToJSON(req.Headers))
	if err != nil {
		return fmt.Errorf("%s: marshal headers: %w", funcName, err)
	}

	metadataJSON, err := json.Marshal(req.GRPCMetadata)
	if err != nil {
		return fmt.Errorf("%s: marshal grpc_metadata: %w", funcName, err)
	}

	query := `UPDATE requests SET collection_id = ?, name = ?, description = ?, protocol = ?, method = ?, url = ?,
		headers = ?, body = ?, body_type = ?,
		auth_type = ?, auth_data = ?,
		grpc_service = ?, grpc_method = ?, grpc_proto_path = ?, grpc_metadata = ?,
		graphql_query = ?, graphql_variables = ?, graphql_schema_path = ?, graphql_operation = ?,
		pre_script = ?, post_script = ?, sort_order = ?, version = ?, is_delete = ?, is_draft = ?,
		created_by = ?, created_at = ?, updated_by = ?, updated_at = ?
		WHERE id = ?`

	_, err = DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		req.CollectionID.String(),
		req.Name,
		req.Description,
		string(req.Protocol),
		string(req.Method),
		req.URL,
		string(headersJSON),
		req.Body,
		string(req.BodyType),
		string(req.AuthType),
		req.AuthData,
		req.GRPCService,
		req.GRPCMethod,
		req.GRPCProtoPath,
		string(metadataJSON),
		req.GraphQLQuery,
		req.GraphQLVariables,
		req.GraphQLSchemaPath,
		req.GraphQLOperation,
		req.PreScript,
		req.PostScript,
		req.SortOrder,
		req.Version,
		boolToInt(req.IsDelete),
		boolToInt(req.IsDraft),
		req.CreatedBy,
		req.CreatedAt.Format(time.RFC3339),
		req.UpdatedBy,
		req.UpdatedAt.Format(time.RFC3339),
		req.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Ignores is_delete: sync has to preserve the field on soft-deleted rows too.
func (r *RequestRepo) GetDescriptionByID(ctx context.Context, id uuid.UUID) (string, error) {
	const funcName = "RequestRepo.GetDescriptionByID"

	var description string
	err := DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		"SELECT description FROM requests WHERE id = ?", id.String()).Scan(&description)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}

	return description, nil
}

// Used only for drafts.
func (r *RequestRepo) DeleteHard(ctx context.Context, id uuid.UUID) error {
	const funcName = "RequestRepo.DeleteHard"
	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, "DELETE FROM requests WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// Called on app startup to clean drafts left over from previous crashes.
func (r *RequestRepo) CleanupDrafts(ctx context.Context) (int, error) {
	const funcName = "RequestRepo.CleanupDrafts"
	res, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, "DELETE FROM requests WHERE is_draft = 1")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}
	return int(n), nil
}

func (r *RequestRepo) UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	const funcName = "RequestRepo.UpdateSortOrder"

	query := `UPDATE requests SET sort_order = ? WHERE id = ?`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, sortOrder, id.String())
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func scanRequest(s scannable) (*entities.Request, error) {
	var (
		req          entities.Request
		idStr        string
		collectionID string
		protocol     string
		method       string
		bodyType     string
		authType     string
		authData     string
		headersJSON  string
		metadataJSON string
		isDelete     int
		isDraft      int
		createdAt    string
		updatedAt    string
	)

	err := s.Scan(
		&idStr,
		&collectionID,
		&req.Name,
		&req.Description,
		&protocol,
		&method,
		&req.URL,
		&headersJSON,
		&req.Body,
		&bodyType,
		&authType,
		&authData,
		&req.GRPCService,
		&req.GRPCMethod,
		&req.GRPCProtoPath,
		&metadataJSON,
		&req.GraphQLQuery,
		&req.GraphQLVariables,
		&req.GraphQLSchemaPath,
		&req.GraphQLOperation,
		&req.PreScript,
		&req.PostScript,
		&req.SortOrder,
		&req.Version,
		&isDelete,
		&isDraft,
		&req.CreatedBy,
		&createdAt,
		&req.UpdatedBy,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	req.ID = uuid.MustParse(idStr)
	req.CollectionID = uuid.MustParse(collectionID)
	req.Protocol = entities.Protocol(protocol)
	req.Method = entities.HTTPMethod(method)
	req.BodyType = entities.BodyType(bodyType)
	req.AuthType = entities.AuthType(authType)
	req.AuthData = authData
	if req.AuthData == "" {
		req.AuthData = "{}"
	}
	req.IsDelete = isDelete != 0
	req.IsDraft = isDraft != 0

	req.Headers, err = unmarshalHeaders(headersJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal headers: %w", err)
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &req.GRPCMetadata); err != nil {
			return nil, fmt.Errorf("unmarshal grpc_metadata: %w", err)
		}
	}
	if req.GRPCMetadata == nil {
		req.GRPCMetadata = make(map[string][]string)
	}

	req.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	req.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &req, nil
}

func headersToJSON(items []entities.HeaderItem) []headerItemJSON {
	result := make([]headerItemJSON, len(items))
	for i, item := range items {
		result[i] = headerItemJSON{Key: item.Key, Value: item.Value, Enabled: item.Enabled}
	}
	return result
}

func unmarshalHeaders(raw string) ([]entities.HeaderItem, error) {
	if raw == "" || raw == "null" || raw == "[]" {
		return []entities.HeaderItem{}, nil
	}

	var items []headerItemJSON
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}

	result := make([]entities.HeaderItem, len(items))
	for i, item := range items {
		result[i] = entities.HeaderItem{Key: item.Key, Value: item.Value, Enabled: item.Enabled}
	}
	return result, nil
}
