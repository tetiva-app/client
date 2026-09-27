package wails

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	publicationv1 "github.com/tetiva-app/proto/go/gophercourier/publication/v1"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// Limits the server enforces on PublishRequest; checked here so the preview can say so first.
const (
	snapshotSizeLimit   = 8 << 20
	snapshotGzipLimit   = 7 << 19 // 3.5 MiB
	maxPublishAsIs      = 500
	maxPublishAsIsBytes = 200
	maxPublishAsIsTotal = 16 << 10
	minPasswordBytes    = 8
	maxPasswordBytes    = 72
)

const (
	largestExamplesListed = 5
	// unpublishRefusalsShown is how many refusals of a pending unpublish the panel waits out before it shows one.
	unpublishRefusalsShown = 3
	// previewLocale stands in for the author's locale, which the content hash leaves out.
	previewLocale = "en"
)

const (
	unavailableNotLoggedIn  = "not_logged_in"
	unavailableNoCapability = "no_capability"
	unavailableNotRoot      = "not_root"
	unavailableOffline      = "offline"

	changesYes     = "yes"
	changesNo      = "no"
	changesUnknown = "unknown"

	publicationActive  = "active"
	publicationRevoked = "revoked"
)

// pendingRetryDelays space out the passes that retry an unpublish the network held back; the last repeats.
var pendingRetryDelays = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

// Plan features the server checks before an unlisted or password publication (server plan catalog).
const (
	featurePublishUnlisted = "publish.unlisted"
	featurePublishPassword = "publish.password"
)

const (
	reasonCollectionNotSynced = "PUBLISH_COLLECTION_NOT_SYNCED"
	// ReasonCollectionSyncing replaces NOT_SYNCED once Publish has queued the tree the server was missing.
	ReasonCollectionSyncing = "PUBLISH_COLLECTION_SYNCING"
)

// CollectionUploader queues the parts of a collection tree the sync server never received.
type CollectionUploader interface {
	QueueUnsyncedTree(ctx context.Context, workspaceID, collectionID string) (int, error)
}

var visibilities = map[string]publicationv1.Visibility{
	"public":   publicationv1.Visibility_VISIBILITY_PUBLIC,
	"unlisted": publicationv1.Visibility_VISIBILITY_UNLISTED,
	"password": publicationv1.Visibility_VISIBILITY_PASSWORD,
}

// PublicationService publishes a root collection to the user's sync server. The local table is a
// cache of what the server said plus the unpublish intent a delete leaves behind.
type PublicationService struct {
	collections collection.Usecase
	requests    request.Usecase
	examples    example.Usecase
	envs        environment.Usecase
	workspaces  workspace.Usecase
	remote      syncsvc.PublicationRemote
	config      sqlite.SyncConfigRepository
	repo        *sqlite.PublicationRepo
	uploader    CollectionUploader

	// spawn starts the pending pass Status ends with; after schedules a retry pass. Tests swap both.
	spawn func(func())
	after func(time.Duration, func()) (stop func() bool)

	pendingMu      sync.Mutex
	pendingRunning bool
	pendingAgain   bool

	retryMu    sync.Mutex
	retryStep  int
	stopRetry  func() bool
	retriesOff bool

	// queuedLast is what queueNotSynced queued last per collection.
	queuedMu   sync.Mutex
	queuedLast map[uuid.UUID]int
}

func NewPublicationService(
	collections collection.Usecase,
	requests request.Usecase,
	examples example.Usecase,
	envs environment.Usecase,
	workspaces workspace.Usecase,
	remote syncsvc.PublicationRemote,
	config sqlite.SyncConfigRepository,
	repo *sqlite.PublicationRepo,
	uploader CollectionUploader,
) *PublicationService {
	return &PublicationService{
		collections: collections,
		requests:    requests,
		examples:    examples,
		envs:        envs,
		workspaces:  workspaces,
		remote:      remote,
		config:      config,
		repo:        repo,
		uploader:    uploader,
		spawn:       func(fn func()) { go fn() },
		after:       func(d time.Duration, fn func()) func() bool { return time.AfterFunc(d, fn).Stop },
		queuedLast:  map[uuid.UUID]int{},
	}
}

func (s *PublicationService) Status(req dto.PublicationStatusRequest) Result[dto.PublicationStatus] {
	ctx := context.Background()

	id, err := parseUUIDField("collectionId", req.CollectionID)
	if err != nil {
		return Err[dto.PublicationStatus](err)
	}
	col, err := s.collections.GetByID(ctx, id)
	if err != nil {
		return Err[dto.PublicationStatus](err)
	}
	st, err := s.status(ctx, col)
	if err != nil {
		return Err[dto.PublicationStatus](err)
	}
	s.spawn(func() { s.ProcessPending(context.Background()) })
	return OK(st)
}

// Plan lets the dialog lock what the plan lacks before the server refuses it; the server still decides.
func (s *PublicationService) Plan(req dto.PublishPlanRequest) Result[dto.PublishPlan] {
	const funcName = "PublicationService.Plan"
	ctx := context.Background()

	id, err := parseUUIDField("collectionId", req.CollectionID)
	if err != nil {
		return Err[dto.PublishPlan](err)
	}
	col, err := s.collections.GetByID(ctx, id)
	if err != nil {
		return Err[dto.PublishPlan](err)
	}
	ws, err := s.workspaces.GetByID(ctx, col.WorkspaceID)
	if err != nil {
		return Err[dto.PublishPlan](err)
	}
	features, err := s.remote.PlanFeatures(ctx, remoteWorkspaceID(ws) == "")
	if err != nil {
		return Err[dto.PublishPlan](unreachable(fmt.Errorf("%s: %w", funcName, err)))
	}
	return OK(dto.PublishPlan{
		Unlisted: slices.Contains(features, featurePublishUnlisted),
		Password: slices.Contains(features, featurePublishPassword),
	})
}

// Preview builds the snapshot without the network; PreviewHash ties a later Publish to what was reviewed.
func (s *PublicationService) Preview(req dto.PublishPreviewRequest) Result[dto.PublishPreview] {
	p, err := s.prepare(context.Background(), req, previewLocale)
	if err != nil {
		return Err[dto.PublishPreview](err)
	}
	out := dto.PublishPreviewFromReport(p.report)
	out.SizeBytes, out.SizeLimitBytes = len(p.raw), snapshotSizeLimit
	out.GzipBytes, out.GzipLimitBytes = len(p.gz), snapshotGzipLimit
	if overSizeLimit(p) {
		out.LargestExamples = largestExamples(p.snapshot)
	}
	out.PreviewHash = p.previewHash
	return OK(out)
}

func (s *PublicationService) Publish(req dto.PublishRequest) Result[dto.PublicationStatus] {
	st, err := s.publish(context.Background(), req)
	if err != nil {
		return Err[dto.PublicationStatus](unreachable(err))
	}
	return OK(st)
}

// Unpublish is a user action: a failure goes back to the user and leaves no deferred intent.
func (s *PublicationService) Unpublish(req dto.UnpublishRequest) Result[dto.PublicationStatus] {
	const funcName = "PublicationService.Unpublish"
	ctx := context.Background()

	id, err := parseUUIDField("collectionId", req.CollectionID)
	if err != nil {
		return Err[dto.PublicationStatus](err)
	}
	col, err := s.collections.GetByID(ctx, id)
	if err != nil {
		return Err[dto.PublicationStatus](err)
	}
	owner, err := s.currentOwner(ctx)
	if err != nil {
		return Err[dto.PublicationStatus](fmt.Errorf("%s: %w", funcName, err))
	}
	if owner == "" {
		return Err[dto.PublicationStatus](unreachable(fmt.Errorf("%s: %w", funcName, ErrNotConnected)))
	}
	row, err := s.repo.Get(ctx, owner, id)
	if err != nil {
		return Err[dto.PublicationStatus](fmt.Errorf("%s: %w", funcName, err))
	}
	if row == nil {
		return Err[dto.PublicationStatus](&domain.NotFoundError{Entity: "publication", ID: id.String()})
	}
	if _, err := s.remote.Unpublish(ctx, row.PublicationID); err != nil {
		return Err[dto.PublicationStatus](unreachable(fmt.Errorf("%s: %w", funcName, err)))
	}
	settleUnpublished(row)
	if err := s.repo.Upsert(ctx, row); err != nil {
		return Err[dto.PublicationStatus](fmt.Errorf("%s: %w", funcName, err))
	}

	st := statusFromRow(row)
	st.Available = col.ParentID == nil
	if !st.Available {
		st.ReasonUnavailable = unavailableNotRoot
	}
	return OK(st)
}

// MarkVariableSecret finds the variable through ListVariables: the usecase has no GetVariable.
func (s *PublicationService) MarkVariableSecret(req dto.MarkSecretRequest) Result[Empty] {
	ctx := context.Background()

	envID, err := parseUUIDField("environmentId", req.EnvironmentID)
	if err != nil {
		return Err[Empty](err)
	}
	varID, err := parseUUIDField("variableId", req.VariableID)
	if err != nil {
		return Err[Empty](err)
	}
	vars, err := s.envs.ListVariables(ctx, envID)
	if err != nil {
		return Err[Empty](err)
	}
	i := slices.IndexFunc(vars, func(v *entities.Variable) bool { return v.ID == varID })
	if i < 0 {
		return Err[Empty](&domain.NotFoundError{Entity: "variable", ID: varID.String()})
	}
	v := vars[i]
	_, err = s.envs.EditVariable(ctx,
		environment.EditVariable{Key: v.Key, Value: v.Value, IsSecret: true, Enabled: v.Enabled},
		environment.EditVariableOpt{VariableID: v.ID, UserID: defaultUserID, Version: v.Version})
	if err != nil {
		return Err[Empty](err)
	}
	return OK(Empty{})
}

// ProcessPending takes down the pages of deleted and nested collections. A call that arrives
// while a pass runs makes that pass go round once more instead of waiting for it.
//
//wails:ignore
func (s *PublicationService) ProcessPending(ctx context.Context) {
	s.pendingMu.Lock()
	if s.pendingRunning {
		s.pendingAgain = true
		s.pendingMu.Unlock()
		return
	}
	s.pendingRunning = true
	s.pendingMu.Unlock()

	for {
		deferred, err := s.processPending(ctx)
		if err != nil {
			slog.Warn("publication: pending pass failed", "err", err)
		}
		s.pendingMu.Lock()
		if !s.pendingAgain {
			s.pendingRunning = false
			s.scheduleRetry(deferred)
			s.pendingMu.Unlock()
			return
		}
		s.pendingAgain = false
		s.pendingMu.Unlock()
	}
}

// WatchDeletes starts a pending pass whenever a delete commits a mark, until ctx ends; a retry
// still waiting then is dropped.
//
//wails:ignore
func (s *PublicationService) WatchDeletes(ctx context.Context) {
	defer s.stopRetries()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.repo.Marked():
			s.ProcessPending(ctx)
		}
	}
}

// scheduleRetry backs off while the network holds a pending unpublish back and starts over once a
// pass gets through.
func (s *PublicationService) scheduleRetry(deferred bool) {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	if s.stopRetry != nil {
		s.stopRetry()
		s.stopRetry = nil
	}
	if !deferred {
		s.retryStep = 0
		return
	}
	if s.retriesOff {
		return
	}
	delay := pendingRetryDelays[min(s.retryStep, len(pendingRetryDelays)-1)]
	s.retryStep++
	s.stopRetry = s.after(delay, func() { s.ProcessPending(context.Background()) })
}

func (s *PublicationService) stopRetries() {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	s.retriesOff = true
	if s.stopRetry != nil {
		s.stopRetry()
		s.stopRetry = nil
	}
}

func (s *PublicationService) status(ctx context.Context, col *entities.Collection) (dto.PublicationStatus, error) {
	const funcName = "PublicationService.status"

	owner, err := s.currentOwner(ctx)
	if err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}
	row, err := s.repo.Get(ctx, owner, col.ID)
	if err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}

	reason, _ := s.availability(ctx, owner)
	if reason == "" {
		pubs, err := s.remote.GetPublications(ctx, []string{col.ID.String()})
		if err != nil {
			reason = unavailableReason(err)
		} else if row, err = s.refresh(ctx, col, owner, row, pubs); err != nil {
			return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
		}
	}
	st := statusFromRow(row)
	switch {
	case reason != "":
		st.ReasonUnavailable, st.Stale = reason, row != nil
		return st, nil
	case col.ParentID != nil:
		st.ReasonUnavailable = unavailableNotRoot
		return st, nil
	}

	st.Available = true
	if err := s.withChanges(ctx, col, row, &st); err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}
	return st, nil
}

func (s *PublicationService) withChanges(ctx context.Context, col *entities.Collection, row *sqlite.PublicationRow, st *dto.PublicationStatus) error {
	changes, envMissing, err := s.hasChanges(ctx, col, row)
	if err != nil {
		return err
	}
	st.HasChanges = changes
	if st.Settings != nil {
		st.Settings.EnvironmentMissing = envMissing
	}
	return nil
}

// List works HasChanges out even offline, unlike Status: the panel counts outdated pages.
func (s *PublicationService) List(req dto.PublicationListRequest) Result[dto.PublicationList] {
	out, err := s.list(context.Background(), req)
	if err != nil {
		return Err[dto.PublicationList](err)
	}
	return OK(out)
}

func (s *PublicationService) list(ctx context.Context, req dto.PublicationListRequest) (dto.PublicationList, error) {
	const funcName = "PublicationService.list"

	out := dto.PublicationList{Items: []dto.PublicationListItem{}}
	wsID, err := parseUUIDField("workspaceId", req.WorkspaceID)
	if err != nil {
		return out, err
	}
	owner, err := s.currentOwner(ctx)
	if err != nil {
		return out, fmt.Errorf("%s: %w", funcName, err)
	}
	if owner == "" {
		out.Reason = unavailableNotLoggedIn
		return out, nil
	}
	roots, err := s.roots(ctx, wsID)
	if err != nil {
		return out, fmt.Errorf("%s: %w", funcName, err)
	}
	if req.Remote {
		if out.Reason, roots, err = s.refreshRoots(ctx, owner, wsID, roots); err != nil {
			return out, fmt.Errorf("%s: %w", funcName, err)
		}
	}

	for _, col := range roots {
		row, err := s.repo.Get(ctx, owner, col.ID)
		if err != nil {
			return out, fmt.Errorf("%s: %w", funcName, err)
		}
		st := statusFromRow(row)
		if !st.Published && !st.PendingUnpublish {
			continue
		}
		st.Available, st.ReasonUnavailable, st.Stale = out.Reason == "", out.Reason, out.Reason != ""
		// An unreadable collection stays listed as unknown instead of failing the list.
		if err := s.withChanges(ctx, col, row, &st); err != nil {
			slog.Warn("publication: list could not compare a page", "collection", col.ID, "err", err)
		}
		out.Items = append(out.Items, dto.PublicationListItem{CollectionID: col.ID.String(), Name: col.Name, Status: st})
	}
	slices.SortFunc(out.Items, func(a, b dto.PublicationListItem) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.CollectionID, b.CollectionID))
	})
	return out, nil
}

// refreshRoots re-reads the roots after the call: one deleted meanwhile keeps its mark.
func (s *PublicationService) refreshRoots(ctx context.Context, owner string, wsID uuid.UUID,
	roots []*entities.Collection) (string, []*entities.Collection, error) {
	if reason, _ := s.availability(ctx, owner); reason != "" {
		return reason, roots, nil
	}
	if len(roots) == 0 {
		return "", roots, nil
	}
	ids := make([]string, 0, len(roots))
	asked := make(map[uuid.UUID]bool, len(roots))
	for _, c := range roots {
		ids = append(ids, c.ID.String())
		asked[c.ID] = true
	}
	pubs, err := s.remote.GetPublications(ctx, ids)
	if err != nil {
		return unavailableReason(err), roots, nil
	}

	live, err := s.roots(ctx, wsID)
	if err != nil {
		return "", nil, err
	}
	for _, col := range live {
		if !asked[col.ID] {
			continue
		}
		row, err := s.repo.Get(ctx, owner, col.ID)
		if err != nil {
			return "", nil, err
		}
		if _, err := s.refresh(ctx, col, owner, row, pubs); err != nil {
			return "", nil, err
		}
	}
	return "", live, nil
}

func (s *PublicationService) roots(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	all, err := s.collections.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(c *entities.Collection) bool { return c.ParentID != nil }), nil
}

// refresh replaces the cached row with the server's record; a collection the server has no record
// of loses its row.
func (s *PublicationService) refresh(ctx context.Context, col *entities.Collection, owner string,
	row *sqlite.PublicationRow, pubs []*publicationv1.Publication) (*sqlite.PublicationRow, error) {
	i := slices.IndexFunc(pubs, func(p *publicationv1.Publication) bool { return p.GetCollectionId() == col.ID.String() })
	if i < 0 {
		if row == nil {
			return nil, nil
		}
		return nil, s.repo.Delete(ctx, owner, col.ID)
	}
	next := rowFromPublication(col, owner, row, pubs[i])
	if err := s.repo.Upsert(ctx, next); err != nil {
		return nil, err
	}
	return next, nil
}

// hasChanges rebuilds the snapshot with the server's settings: those are what the page was built
// from, whoever published it and on whatever device.
func (s *PublicationService) hasChanges(ctx context.Context, col *entities.Collection, row *sqlite.PublicationRow) (string, bool, error) {
	if row == nil || row.Status != publicationActive || !row.CanManage || row.Settings == nil {
		return changesUnknown, false, nil
	}
	env, vars, err := s.environment(ctx, col.WorkspaceID, row.Settings.EnvironmentID)
	var valErr *domain.ValidationError
	if errors.As(err, &valErr) {
		return changesUnknown, true, nil
	}
	if err != nil {
		return "", false, err
	}
	snap, _, err := s.snapshot(ctx, col, env, vars, row.Settings.IncludeScripts, row.Settings.PublishAsIs, previewLocale)
	if err != nil {
		return "", false, err
	}
	hash, err := snapshotjson.ContentHash(snap)
	if err != nil {
		return "", false, err
	}
	if hash == row.ContentHash {
		return changesNo, false, nil
	}
	return changesYes, false, nil
}

func (s *PublicationService) publish(ctx context.Context, req dto.PublishRequest) (dto.PublicationStatus, error) {
	const funcName = "PublicationService.publish"

	visibility, err := validatePublishFields(req)
	if err != nil {
		return dto.PublicationStatus{}, err
	}
	p, err := s.prepare(ctx, dto.PublishPreviewRequest{
		CollectionID: req.CollectionID, WorkspaceID: req.WorkspaceID, EnvironmentID: req.EnvironmentID,
		IncludeScripts: req.IncludeScripts, PublishAsIs: req.PublishAsIs,
	}, req.Locale)
	if err != nil {
		return dto.PublicationStatus{}, err
	}
	if n := len(p.report.Errors); n > 0 {
		return dto.PublicationStatus{}, &domain.ValidationError{Fields: map[string]string{
			"snapshot": fmt.Sprintf("%d problems in the preview block publishing", n),
		}}
	}
	if p.previewHash != req.PreviewHash {
		return dto.PublicationStatus{}, &domain.ConflictError{Entity: "publication preview", ID: p.collection.ID.String()}
	}
	if err := checkOverrideLimits(p.report.AcceptedOverrides); err != nil {
		return dto.PublicationStatus{}, err
	}
	if len(p.report.Warnings) > 0 && !req.AcknowledgedWarnings {
		return dto.PublicationStatus{}, &domain.ValidationError{Fields: map[string]string{
			"acknowledgedWarnings": "review the warnings in the preview first",
		}}
	}
	if err := checkSnapshotSize(p); err != nil {
		return dto.PublicationStatus{}, err
	}

	owner, err := s.currentOwner(ctx)
	if err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}
	switch reason, err := s.availability(ctx, owner); {
	case err != nil:
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	case reason != "":
		return dto.PublicationStatus{}, &domain.ValidationError{Fields: map[string]string{
			"collectionId": "this server doesn't support publishing",
		}}
	}
	ws, err := s.workspaces.GetByID(ctx, p.collection.WorkspaceID)
	if err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}

	settings := &sqlite.PublicationSettings{IncludeScripts: req.IncludeScripts, PublishAsIs: p.report.AcceptedOverrides}
	if p.environment != nil {
		settings.EnvironmentID, settings.EnvironmentName = p.environment.ID.String(), p.environment.Name
		// The scrubbed name is what the preview checked and the page shows; settings must not carry more.
		if p.snapshot.Environment != nil {
			settings.EnvironmentName = p.snapshot.Environment.Name
		}
	}
	var password *string
	if visibility == publicationv1.Visibility_VISIBILITY_PASSWORD {
		password = req.Password
	}
	pub, err := s.remote.Publish(ctx, publicationv1.PublishRequest_builder{
		CollectionId: p.collection.ID.String(),
		WorkspaceId:  remoteWorkspaceID(ws),
		Visibility:   visibility,
		Password:     password,
		SnapshotGz:   p.gz,
		ContentHash:  p.contentHash,
		Title:        p.snapshot.Collection.Name,
		Locale:       req.Locale,
		Settings: publicationv1.PublishSettings_builder{
			EnvironmentId:   settings.EnvironmentID,
			EnvironmentName: settings.EnvironmentName,
			IncludeScripts:  settings.IncludeScripts,
			PublishAsIs:     settings.PublishAsIs,
		}.Build(),
		ConfirmMakePublic: req.ConfirmMakePublic,
	}.Build())
	if err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, s.queueNotSynced(ctx, ws, p.collection, err))
	}

	prev, err := s.repo.Get(ctx, owner, p.collection.ID)
	if err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}
	row := rowFromPublication(p.collection, owner, prev, pub)
	row.Settings, row.CanManage, row.PendingUnpublish = settings, true, false
	row.UnpublishAttempts, row.UnpublishError = 0, ""
	if err := s.repo.Upsert(ctx, row); err != nil {
		return dto.PublicationStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}

	st := statusFromRow(row)
	st.Available = true
	if row.ContentHash == p.contentHash {
		st.HasChanges = changesNo
	}
	return st, nil
}

// queueNotSynced answers NOT_SYNCED for a linked workspace by queueing what the server lacks: a tree
// written before the workspace was linked would otherwise never go out, and waiting would not help.
func (s *PublicationService) queueNotSynced(ctx context.Context, ws *entities.Workspace, col *entities.Collection, err error) error {
	if errorReason(err) != reasonCollectionNotSynced || remoteWorkspaceID(ws) == "" {
		return err
	}
	n, qErr := s.uploader.QueueUnsyncedTree(ctx, ws.ID.String(), col.ID.String())
	switch {
	case qErr != nil:
		return errors.Join(err, qErr)
	case n > 0 && s.progressed(col.ID, n):
		return &domain.ReasonError{Reason: ReasonCollectionSyncing, Err: err}
	}
	return err
}

// progressed records n; as many rows missing as last time means the server refused what went up.
func (s *PublicationService) progressed(collectionID uuid.UUID, n int) bool {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()
	prev, seen := s.queuedLast[collectionID]
	s.queuedLast[collectionID] = n
	return !seen || n < prev
}

type preparedSnapshot struct {
	collection  *entities.Collection
	environment *entities.Environment
	snapshot    *publication.Snapshot
	report      publication.Report
	raw         []byte
	gz          []byte
	contentHash string
	previewHash string
}

func (s *PublicationService) prepare(ctx context.Context, req dto.PublishPreviewRequest, locale string) (*preparedSnapshot, error) {
	const funcName = "PublicationService.prepare"

	colID, err := parseUUIDField("collectionId", req.CollectionID)
	if err != nil {
		return nil, err
	}
	wsID, err := parseUUIDField("workspaceId", req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	col, err := s.collections.GetByID(ctx, colID)
	if err != nil {
		return nil, err
	}
	if col.ParentID != nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"collectionId": "only top-level collections can be published"}}
	}
	if col.WorkspaceID != wsID {
		return nil, &domain.ValidationError{Fields: map[string]string{"workspaceId": "the collection is in another workspace"}}
	}
	env, vars, err := s.environment(ctx, col.WorkspaceID, req.EnvironmentID)
	if err != nil {
		return nil, err
	}

	snap, report, err := s.snapshot(ctx, col, env, vars, req.IncludeScripts, req.PublishAsIs, locale)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	raw, err := snapshotjson.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	gz, err := snapshotjson.Gzip(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	contentHash, err := snapshotjson.ContentHash(snap)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	digest := sha256.Sum256([]byte(contentHash + "\n" + publication.ReportDigest(report)))

	return &preparedSnapshot{
		collection: col, environment: env, snapshot: snap, report: report, raw: raw, gz: gz,
		contentHash: contentHash, previewHash: hex.EncodeToString(digest[:]),
	}, nil
}

func (s *PublicationService) snapshot(ctx context.Context, col *entities.Collection, env *entities.Environment,
	vars []*entities.Variable, includeScripts bool, publishAsIs []string, locale string) (*publication.Snapshot, publication.Report, error) {
	all, err := s.collections.List(ctx, collection.ListOpt{WorkspaceID: col.WorkspaceID})
	if err != nil {
		return nil, publication.Report{}, err
	}
	subtree := filterSubtree(col.ID, all)

	var requests []*entities.Request
	for _, c := range subtree {
		list, err := s.requests.List(ctx, request.ListOpt{CollectionID: c.ID})
		if err != nil {
			return nil, publication.Report{}, err
		}
		requests = append(requests, list...)
	}
	examples := make(map[uuid.UUID][]*entities.ResponseExample, len(requests))
	for _, r := range requests {
		list, err := s.examples.ListByRequest(ctx, r.ID)
		if err != nil {
			return nil, publication.Report{}, err
		}
		examples[r.ID] = list
	}

	return publication.Build(publication.BuildInput{
		Root: col, Collections: subtree, Requests: requests, Examples: examples,
		Environment: env, Variables: vars, IncludeScripts: includeScripts,
		Generator: constants.AppName + " " + constants.AppVersion, Locale: locale, PublishAsIs: publishAsIs,
	})
}

// environment returns nil for "" (publishing without one); anything outside the collection's
// workspace is a ValidationError, which the caller may read as a deleted environment.
func (s *PublicationService) environment(ctx context.Context, workspaceID uuid.UUID, raw string) (*entities.Environment, []*entities.Variable, error) {
	if raw == "" {
		return nil, nil, nil
	}
	id, err := parseUUIDField("environmentId", raw)
	if err != nil {
		return nil, nil, err
	}
	env, err := s.envs.GetByID(ctx, id)
	var notFound *domain.NotFoundError
	if errors.As(err, &notFound) {
		return nil, nil, &domain.ValidationError{Fields: map[string]string{"environmentId": "environment not found"}}
	}
	if err != nil {
		return nil, nil, err
	}
	if env.WorkspaceID != workspaceID {
		return nil, nil, &domain.ValidationError{Fields: map[string]string{"environmentId": "the environment is in another workspace"}}
	}
	vars, err := s.envs.ListVariables(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return env, vars, nil
}

// currentOwner is "" when signed out. Every read goes by the owner, so the rows of another account or
// server stay out of sight but stay put: a delete under this account still marks them.
func (s *PublicationService) currentOwner(ctx context.Context) (string, error) {
	cfg, err := s.config.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("read sync config: %w", err)
	}
	if cfg == nil || !cfg.Enabled || cfg.UserEmail == "" {
		return "", nil
	}
	return cfg.ServerURL + "\n" + cfg.UserEmail, nil
}

// availability returns "" when publishing can go ahead; the error is set for a missing session or
// an unreachable server.
func (s *PublicationService) availability(ctx context.Context, owner string) (string, error) {
	if owner == "" {
		return unavailableNotLoggedIn, ErrNotConnected
	}
	ok, err := s.remote.ServerSupportsPublish(ctx)
	switch {
	case err != nil:
		return unavailableReason(err), err
	case !ok:
		return unavailableNoCapability, nil
	}
	return "", nil
}

func unavailableReason(err error) string {
	if errors.Is(err, ErrNotConnected) || errors.Is(err, syncsvc.ErrAuthExpired) || errors.Is(err, syncsvc.ErrSessionReplaced) {
		return unavailableNotLoggedIn
	}
	return unavailableOffline
}

// processPending reports deferred when the network held an unpublish back.
func (s *PublicationService) processPending(ctx context.Context) (bool, error) {
	owner, err := s.currentOwner(ctx)
	if err != nil {
		return false, err
	}
	if owner == "" {
		return false, nil
	}
	rows, err := s.repo.List(ctx, owner)
	if err != nil {
		return false, err
	}
	var pending []*sqlite.PublicationRow
	for _, row := range rows {
		if !row.PendingUnpublish {
			kept, err := s.reconcile(ctx, row)
			if err != nil {
				return false, err
			}
			if !kept {
				continue
			}
		}
		if row.PendingUnpublish {
			pending = append(pending, row)
		}
	}
	for _, row := range pending {
		stop, err := s.unpublishPending(ctx, row)
		if err != nil || stop {
			return stop, err
		}
	}
	return false, nil
}

// reconcile catches what no delete transaction marked: a root moved into a folder, a collection
// that died with its parent, a cloud publication, which the server takes down itself. A dead local
// collection is marked again: a refresh or publish racing its delete overwrites the mark.
func (s *PublicationService) reconcile(ctx context.Context, row *sqlite.PublicationRow) (bool, error) {
	ws, err := s.liveWorkspace(ctx, row.WorkspaceID)
	if err != nil {
		return false, err
	}
	var col *entities.Collection
	if ws != nil {
		if col, err = s.liveCollection(ctx, row.CollectionID); err != nil {
			return false, err
		}
	}
	cloud := row.Cloud
	if col == nil && !cloud {
		if cloud, err = s.repo.WorkspaceLinked(ctx, row.WorkspaceID); err != nil {
			return false, err
		}
	}
	switch {
	case col == nil && cloud:
		return false, s.repo.Delete(ctx, row.OwnerKey, row.CollectionID)
	case col == nil, col.ParentID != nil && row.Status != publicationRevoked:
		row.PendingUnpublish = true
		return true, s.repo.Upsert(ctx, row)
	}
	return true, nil
}

// unpublishPending reports stop when the server is out of reach, so the rest wait for the next pass.
// Any other refusal is kept on the row and counted; the intent stays.
func (s *PublicationService) unpublishPending(ctx context.Context, row *sqlite.PublicationRow) (bool, error) {
	_, err := s.remote.Unpublish(ctx, row.PublicationID)
	if err != nil {
		st, fromServer := status.FromError(err)
		switch {
		case fromServer && (st.Code() == codes.PermissionDenied || st.Code() == codes.NotFound):
		case !fromServer || st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded || st.Code() == codes.Canceled:
			slog.Info("publication: unpublish deferred", "collection", row.CollectionID, "err", err)
			return true, nil
		default:
			slog.Warn("publication: unpublish refused", "collection", row.CollectionID, "err", err)
			row.UnpublishAttempts++
			row.UnpublishError = serverMessage(err)
			return false, s.repo.Upsert(ctx, row)
		}
	}

	ws, err := s.liveWorkspace(ctx, row.WorkspaceID)
	if err != nil {
		return false, err
	}
	var col *entities.Collection
	if ws != nil {
		if col, err = s.liveCollection(ctx, row.CollectionID); err != nil {
			return false, err
		}
	}
	if col == nil {
		return false, s.repo.Delete(ctx, row.OwnerKey, row.CollectionID)
	}
	settleUnpublished(row)
	return false, s.repo.Upsert(ctx, row)
}

// serverMessage drops the wrapping status.FromError keeps in Message; the code stands in for an
// empty message, which the panel would read as no refusal at all.
func serverMessage(err error) string {
	var se interface{ GRPCStatus() *status.Status }
	if !errors.As(err, &se) {
		return err.Error()
	}
	st := se.GRPCStatus()
	return cmp.Or(st.Message(), st.Code().String())
}

func settleUnpublished(row *sqlite.PublicationRow) {
	row.Status, row.PendingUnpublish = publicationRevoked, false
	row.UnpublishAttempts, row.UnpublishError = 0, ""
}

func (s *PublicationService) liveWorkspace(ctx context.Context, id uuid.UUID) (*entities.Workspace, error) {
	ws, err := s.workspaces.GetByID(ctx, id)
	var notFound *domain.NotFoundError
	if errors.As(err, &notFound) {
		return nil, nil
	}
	return ws, err
}

func (s *PublicationService) liveCollection(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	col, err := s.collections.GetByID(ctx, id)
	var notFound *domain.NotFoundError
	if errors.As(err, &notFound) {
		return nil, nil
	}
	return col, err
}

func rowFromPublication(col *entities.Collection, owner string, prev *sqlite.PublicationRow, pub *publicationv1.Publication) *sqlite.PublicationRow {
	next := &sqlite.PublicationRow{}
	if prev != nil {
		*next = *prev
	}
	next.CollectionID, next.WorkspaceID, next.OwnerKey = col.ID, col.WorkspaceID, owner
	next.PublicationID, next.Cloud = pub.GetId(), pub.GetWorkspaceId() != ""
	next.Slug, next.PublicURL = pub.GetSlug(), pub.GetPublicUrl()
	next.Visibility = visibilityName(pub.GetVisibility())
	next.Status = ""
	switch pub.GetStatus() {
	case publicationv1.PublicationStatus_PUBLICATION_STATUS_ACTIVE:
		next.Status = publicationActive
	case publicationv1.PublicationStatus_PUBLICATION_STATUS_REVOKED:
		next.Status = publicationRevoked
	}
	next.ContentHash = pub.GetContentHash()
	next.Revision = int(pub.GetRevision())
	next.Blocked, next.BlockedReason, next.Badge = pub.GetBlocked(), pub.GetBlockedReason(), pub.GetBadge()
	next.CanManage = pub.GetCanManage()
	next.Counters = sqlite.PublicationCounters{Views: pub.GetViewsTotal(), Imports: pub.GetImportsTotal(), Downloads: pub.GetDownloadsTotal()}
	if pub.HasSettings() {
		ps := pub.GetSettings()
		next.Settings = &sqlite.PublicationSettings{
			EnvironmentID: ps.GetEnvironmentId(), EnvironmentName: ps.GetEnvironmentName(),
			IncludeScripts: ps.GetIncludeScripts(), PublishAsIs: slices.Clone(ps.GetPublishAsIs()),
		}
	}
	next.ServerUpdatedAt = time.Time{}
	if pub.HasUpdatedAt() {
		next.ServerUpdatedAt = pub.GetUpdatedAt().AsTime()
	}
	next.RefreshedAt = time.Now()
	return next
}

func statusFromRow(row *sqlite.PublicationRow) dto.PublicationStatus {
	st := dto.PublicationStatus{HasChanges: changesUnknown}
	if row == nil {
		return st
	}
	st.Published = row.Status != publicationRevoked
	st.CanManage = row.CanManage
	st.Slug, st.PublicURL, st.Visibility = row.Slug, row.PublicURL, row.Visibility
	st.Revision = row.Revision
	if !row.ServerUpdatedAt.IsZero() {
		st.UpdatedAt = row.ServerUpdatedAt.UTC().Format(time.RFC3339)
	}
	st.Blocked, st.BlockedReason, st.Badge = row.Blocked, row.BlockedReason, row.Badge
	st.Counters = dto.PublicationCounters(row.Counters)
	st.PendingUnpublish = row.PendingUnpublish
	if row.PendingUnpublish && row.UnpublishAttempts >= unpublishRefusalsShown {
		st.UnpublishError = row.UnpublishError
	}
	if row.CanManage && row.Settings != nil {
		st.Settings = &dto.PublishSettings{
			EnvironmentID: row.Settings.EnvironmentID, EnvironmentName: row.Settings.EnvironmentName,
			IncludeScripts: row.Settings.IncludeScripts, PublishAsIs: nonNilSelectors(row.Settings.PublishAsIs),
		}
	}
	return st
}

func validatePublishFields(req dto.PublishRequest) (publicationv1.Visibility, error) {
	fields := map[string]string{}
	visibility, ok := visibilities[req.Visibility]
	if !ok {
		fields["visibility"] = "must be public, unlisted or password"
	}
	if req.Locale != "ru" && req.Locale != "en" {
		fields["locale"] = "must be ru or en"
	}
	if visibility == publicationv1.Visibility_VISIBILITY_PASSWORD && req.Password != nil &&
		(len(*req.Password) < minPasswordBytes || len(*req.Password) > maxPasswordBytes) {
		fields["password"] = fmt.Sprintf("must be %d-%d bytes", minPasswordBytes, maxPasswordBytes)
	}
	if len(fields) > 0 {
		return 0, &domain.ValidationError{Fields: fields}
	}
	return visibility, nil
}

func checkOverrideLimits(selectors []string) error {
	if len(selectors) > maxPublishAsIs {
		return &domain.ValidationError{Fields: map[string]string{
			"publishAsIs": fmt.Sprintf("at most %d values can be published as is", maxPublishAsIs),
		}}
	}
	total := 0
	for _, sel := range selectors {
		if len(sel) > maxPublishAsIsBytes {
			return &domain.ValidationError{Fields: map[string]string{
				"publishAsIs": fmt.Sprintf("a selector is longer than %d bytes", maxPublishAsIsBytes),
			}}
		}
		total += len(sel)
	}
	if total > maxPublishAsIsTotal {
		return &domain.ValidationError{Fields: map[string]string{
			"publishAsIs": fmt.Sprintf("the selectors exceed %d bytes in total", maxPublishAsIsTotal),
		}}
	}
	return nil
}

func overSizeLimit(p *preparedSnapshot) bool {
	return len(p.raw) > snapshotSizeLimit || len(p.gz) > snapshotGzipLimit
}

func checkSnapshotSize(p *preparedSnapshot) error {
	if !overSizeLimit(p) {
		return nil
	}
	msg := fmt.Sprintf("the collection is %s (%s compressed), the limit is %s (%s compressed)",
		formatSize(len(p.raw)), formatSize(len(p.gz)), formatSize(snapshotSizeLimit), formatSize(snapshotGzipLimit))
	if largest := largestExamples(p.snapshot); len(largest) > 0 {
		names := make([]string, 0, len(largest))
		for _, e := range largest {
			names = append(names, fmt.Sprintf("%s (%s)", e.Path, formatSize(e.Bytes)))
		}
		msg += "; largest examples: " + strings.Join(names, ", ")
	}
	return &domain.ValidationError{Fields: map[string]string{"snapshot": msg}}
}

func largestExamples(snap *publication.Snapshot) []dto.SizedExample {
	var all []dto.SizedExample
	var walk func(f *publication.Folder, names []string)
	walk = func(f *publication.Folder, names []string) {
		for _, item := range f.Items {
			switch {
			case item.Folder != nil:
				walk(item.Folder, append(slices.Clip(names), item.Folder.Name))
			case item.Request != nil:
				for _, e := range item.Request.Examples {
					path := strings.Join(append(slices.Clip(names), item.Request.Name, e.Name), " / ")
					all = append(all, dto.SizedExample{Path: path, Bytes: len(e.Body)})
				}
			}
		}
	}
	walk(&snap.Collection.Folder, nil)
	slices.SortFunc(all, func(a, b dto.SizedExample) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.Path, b.Path))
	})
	return slices.Clone(all[:min(len(all), largestExamplesListed)])
}

func formatSize(n int) string {
	if n < 1<<20 {
		return fmt.Sprintf("%.0f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
}

func visibilityName(v publicationv1.Visibility) string {
	for name, pv := range visibilities {
		if pv == v {
			return name
		}
	}
	return ""
}

func remoteWorkspaceID(ws *entities.Workspace) string {
	if ws == nil || ws.RemoteWorkspaceID == nil {
		return ""
	}
	return *ws.RemoteWorkspaceID
}

func nonNilSelectors(in []string) []string {
	if in == nil {
		return []string{}
	}
	return slices.Clone(in)
}

func parseUUIDField(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, &domain.ValidationError{Fields: map[string]string{field: "invalid UUID"}}
	}
	return id, nil
}
