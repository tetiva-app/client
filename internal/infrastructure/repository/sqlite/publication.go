package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// The server moves a local publication into a linked workspace on its next read, so a mapping, even
// on a soft-deleted workspace, counts as cloud before a refresh sets the flag.
const linkedWorkspacesSQL = `SELECT id FROM workspaces WHERE remote_workspace_id <> ''`

const publicationColumns = `collection_id, workspace_id, owner_key, publication_id, slug, public_url, visibility, status,
	content_hash, revision, blocked, blocked_reason, badge, can_manage, settings, counters, pending_unpublish,
	server_updated_at, refreshed_at, created_at, updated_at, cloud, unpublish_attempts, unpublish_error`

// PublicationRow is what the client knows about the publication of one collection. WorkspaceID is
// the local workspace (collection.WorkspaceID), never RemoteWorkspaceID: the markers match on it.
// Cloud is the server's word, which outlives an unlink or an org switch clearing the mapping.
type PublicationRow struct {
	CollectionID     uuid.UUID
	WorkspaceID      uuid.UUID
	OwnerKey         string
	PublicationID    string
	Slug             string
	PublicURL        string
	Visibility       string
	Status           string
	ContentHash      string
	BlockedReason    string
	Revision         int
	Blocked          bool
	Badge            bool
	CanManage        bool
	PendingUnpublish bool
	Cloud            bool
	// UnpublishAttempts counts the server's refusals of the pending unpublish; UnpublishError is the last one.
	UnpublishAttempts int
	UnpublishError    string
	Settings          *PublicationSettings
	Counters          PublicationCounters
	ServerUpdatedAt   time.Time
	RefreshedAt       time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PublicationSettings struct {
	EnvironmentID   string
	EnvironmentName string
	IncludeScripts  bool
	PublishAsIs     []string
}

type PublicationCounters struct {
	Views     int64
	Imports   int64
	Downloads int64
}

type publicationSettingsJSON struct {
	EnvironmentID   string   `json:"environmentId"`
	EnvironmentName string   `json:"environmentName"`
	IncludeScripts  bool     `json:"includeScripts"`
	PublishAsIs     []string `json:"publishAsIs"`
}

type publicationCountersJSON struct {
	Views     int64 `json:"views"`
	Imports   int64 `json:"imports"`
	Downloads int64 `json:"downloads"`
}

// PublicationRepo keeps the local publication cache. Like auth_tokens, the table never syncs and
// its rows are deleted physically.
type PublicationRepo struct {
	db     *sql.DB
	marked chan struct{}
}

func NewPublicationRepo(db *sql.DB) *PublicationRepo {
	return &PublicationRepo{db: db, marked: make(chan struct{}, 1)}
}

// Marked receives once a delete that marked a row has committed; marks in a row coalesce into one.
func (r *PublicationRepo) Marked() <-chan struct{} {
	return r.marked
}

func (r *PublicationRepo) signalMarked(ctx context.Context, res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n > 0 {
		AfterCommit(ctx, func() {
			select {
			case r.marked <- struct{}{}:
			default:
			}
		})
	}
	return nil
}

// Get returns nil, nil when the account has no row for the collection.
func (r *PublicationRepo) Get(ctx context.Context, ownerKey string, collectionID uuid.UUID) (*PublicationRow, error) {
	const funcName = "PublicationRepo.Get"

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+publicationColumns+` FROM publications WHERE collection_id = ? AND owner_key = ?`,
		collectionID.String(), ownerKey)
	p, err := scanPublication(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return p, nil
}

func (r *PublicationRepo) List(ctx context.Context, ownerKey string) ([]*PublicationRow, error) {
	const funcName = "PublicationRepo.List"

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx,
		`SELECT `+publicationColumns+` FROM publications WHERE owner_key = ? ORDER BY created_at, collection_id`, ownerKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var out []*PublicationRow
	for rows.Next() {
		p, err := scanPublication(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", funcName, err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", funcName, err)
	}
	return out, nil
}

// Upsert writes created_at only on insert and updated_at always.
func (r *PublicationRepo) Upsert(ctx context.Context, p *PublicationRow) error {
	const funcName = "PublicationRepo.Upsert"

	var settings any
	if p.Settings != nil {
		asIs := p.Settings.PublishAsIs
		if asIs == nil {
			asIs = []string{}
		}
		raw, err := json.Marshal(publicationSettingsJSON{
			EnvironmentID: p.Settings.EnvironmentID, EnvironmentName: p.Settings.EnvironmentName,
			IncludeScripts: p.Settings.IncludeScripts, PublishAsIs: asIs,
		})
		if err != nil {
			return fmt.Errorf("%s: settings: %w", funcName, err)
		}
		settings = string(raw)
	}
	counters, err := json.Marshal(publicationCountersJSON(p.Counters))
	if err != nil {
		return fmt.Errorf("%s: counters: %w", funcName, err)
	}
	now := formatAuthTime(time.Now())

	_, err = DBTXFromContext(ctx, r.db).ExecContext(ctx,
		`INSERT INTO publications (`+publicationColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (collection_id, owner_key) DO UPDATE SET
			workspace_id = excluded.workspace_id, publication_id = excluded.publication_id,
			slug = excluded.slug, public_url = excluded.public_url, visibility = excluded.visibility, status = excluded.status,
			content_hash = excluded.content_hash, revision = excluded.revision, blocked = excluded.blocked,
			blocked_reason = excluded.blocked_reason, badge = excluded.badge, can_manage = excluded.can_manage,
			settings = excluded.settings, counters = excluded.counters, pending_unpublish = excluded.pending_unpublish,
			server_updated_at = excluded.server_updated_at, refreshed_at = excluded.refreshed_at, updated_at = excluded.updated_at,
			cloud = excluded.cloud, unpublish_attempts = excluded.unpublish_attempts, unpublish_error = excluded.unpublish_error`,
		p.CollectionID.String(), p.WorkspaceID.String(), p.OwnerKey, p.PublicationID, p.Slug, p.PublicURL, p.Visibility,
		p.Status, p.ContentHash, p.Revision, boolToInt(p.Blocked), p.BlockedReason, boolToInt(p.Badge),
		boolToInt(p.CanManage), settings, string(counters), boolToInt(p.PendingUnpublish),
		formatAuthTime(p.ServerUpdatedAt), formatAuthTime(p.RefreshedAt), now, now, boolToInt(p.Cloud), p.UnpublishAttempts, p.UnpublishError)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (r *PublicationRepo) Delete(ctx context.Context, ownerKey string, collectionID uuid.UUID) error {
	const funcName = "PublicationRepo.Delete"

	if _, err := DBTXFromContext(ctx, r.db).ExecContext(ctx,
		`DELETE FROM publications WHERE collection_id = ? AND owner_key = ?`, collectionID.String(), ownerKey); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// MarkPendingUnpublish marks the rows of every account, but only local publications: the server
// takes a deleted cloud collection down itself. A collection without a row is a no-op.
func (r *PublicationRepo) MarkPendingUnpublish(ctx context.Context, collectionIDs []uuid.UUID) error {
	const funcName = "PublicationRepo.MarkPendingUnpublish"

	if len(collectionIDs) == 0 {
		return nil
	}
	args := []any{formatAuthTime(time.Now())}
	for _, id := range collectionIDs {
		args = append(args, id.String())
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(collectionIDs)), ", ")
	res, err := DBTXFromContext(ctx, r.db).ExecContext(ctx,
		`UPDATE publications SET pending_unpublish = 1, updated_at = ?
		WHERE collection_id IN (`+placeholders+`) AND cloud = 0 AND workspace_id NOT IN (`+linkedWorkspacesSQL+`)`, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if err := r.signalMarked(ctx, res); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// MarkPendingUnpublishWorkspace leaves cloud publications up: deleting the local copy of a cloud
// workspace keeps its data on the server.
func (r *PublicationRepo) MarkPendingUnpublishWorkspace(ctx context.Context, workspaceID uuid.UUID) error {
	const funcName = "PublicationRepo.MarkPendingUnpublishWorkspace"

	res, err := DBTXFromContext(ctx, r.db).ExecContext(ctx,
		`UPDATE publications SET pending_unpublish = 1, updated_at = ?
		WHERE workspace_id = ? AND cloud = 0 AND workspace_id NOT IN (`+linkedWorkspacesSQL+`)`,
		formatAuthTime(time.Now()), workspaceID.String())
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if err := r.signalMarked(ctx, res); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// WorkspaceLinked reads the mapping of a soft-deleted workspace too.
func (r *PublicationRepo) WorkspaceLinked(ctx context.Context, workspaceID uuid.UUID) (bool, error) {
	const funcName = "PublicationRepo.WorkspaceLinked"

	var linked bool
	if err := DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		`SELECT EXISTS (`+linkedWorkspacesSQL+` AND id = ?)`, workspaceID.String()).Scan(&linked); err != nil {
		return false, fmt.Errorf("%s: %w", funcName, err)
	}
	return linked, nil
}

type publicationScanner interface {
	Scan(dest ...any) error
}

func scanPublication(s publicationScanner) (*PublicationRow, error) {
	var (
		p                                     PublicationRow
		collectionID, workspaceID             string
		blocked, badge, canManage, pending    int
		cloud                                 int
		settings                              sql.NullString
		counters                              string
		serverUpdated, refreshed, created, up string
	)
	if err := s.Scan(&collectionID, &workspaceID, &p.OwnerKey, &p.PublicationID, &p.Slug, &p.PublicURL, &p.Visibility,
		&p.Status, &p.ContentHash, &p.Revision, &blocked, &p.BlockedReason, &badge, &canManage, &settings, &counters,
		&pending, &serverUpdated, &refreshed, &created, &up, &cloud, &p.UnpublishAttempts, &p.UnpublishError); err != nil {
		return nil, err
	}

	var err error
	if p.CollectionID, err = uuid.Parse(collectionID); err != nil {
		return nil, fmt.Errorf("parse collection_id: %w", err)
	}
	if p.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return nil, fmt.Errorf("parse workspace_id: %w", err)
	}
	p.Blocked, p.Badge, p.CanManage, p.PendingUnpublish = blocked != 0, badge != 0, canManage != 0, pending != 0
	p.Cloud = cloud != 0

	if settings.Valid {
		var sj publicationSettingsJSON
		if err := json.Unmarshal([]byte(settings.String), &sj); err != nil {
			return nil, fmt.Errorf("parse settings: %w", err)
		}
		if sj.PublishAsIs == nil {
			sj.PublishAsIs = []string{}
		}
		p.Settings = &PublicationSettings{
			EnvironmentID: sj.EnvironmentID, EnvironmentName: sj.EnvironmentName,
			IncludeScripts: sj.IncludeScripts, PublishAsIs: sj.PublishAsIs,
		}
	}
	var cj publicationCountersJSON
	if err := json.Unmarshal([]byte(counters), &cj); err != nil {
		return nil, fmt.Errorf("parse counters: %w", err)
	}
	p.Counters = PublicationCounters(cj)

	for _, f := range []struct {
		dst *time.Time
		src string
	}{{&p.ServerUpdatedAt, serverUpdated}, {&p.RefreshedAt, refreshed}, {&p.CreatedAt, created}, {&p.UpdatedAt, up}} {
		if *f.dst, err = parseAuthTime(f.src); err != nil {
			return nil, fmt.Errorf("parse time %q: %w", f.src, err)
		}
	}
	return &p, nil
}
