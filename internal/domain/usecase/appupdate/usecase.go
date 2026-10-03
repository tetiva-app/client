package appupdate

import (
	"context"
	"sync"

	"github.com/tetiva-app/client/internal/domain/entities"
)

const (
	ReasonDevBuild         = "dev_build"
	ReasonTranslocated     = "translocated"
	ReasonReadOnly         = "read_only_location"
	ReasonNotInstalledCopy = "not_installed_copy"
	ReasonCodesignFailed   = "codesign_failed"
	ReasonNoArtifact       = "no_artifact"
	ReasonDisabled         = "disabled_by_manifest"
	ReasonBadManifest      = "bad_manifest"
	ReasonChecksum         = "checksum_mismatch"
	ReasonNetwork          = "network"
	ReasonInstallFailed    = "install_failed"
)

type Usecase interface {
	Start(ctx context.Context)
	Status() entities.UpdateState
	Check(ctx context.Context) entities.UpdateState
	Download(ctx context.Context) entities.UpdateState
	Cancel()
	Apply(ctx context.Context, restore entities.RestoreTabs) error
	TakeRestore() (entities.RestoreTabs, bool)
}

type ManifestSource interface {
	Fetch(ctx context.Context) ([]byte, error)
}

// ManifestCodec verifies the manifest signature before it parses anything.
type ManifestCodec interface {
	Parse(raw []byte) (entities.Release, error)
}

type Downloader interface {
	Download(ctx context.Context, a entities.Artifact, dst string, progress func(received int64)) error
}

type Installer interface {
	Kind(ctx context.Context) (entities.InstallKind, string)
	Verify(ctx context.Context, r entities.Release, file string) error
	Apply(ctx context.Context, r entities.Release, file string) error
}

type Store interface {
	StagePath(r entities.Release, a entities.Artifact) string
	SaveStaged(r entities.Release, manifest []byte, file string) error
	Staged() (manifest []byte, file string, ok bool)
	DropStaged() error
	MarkAttempt(version string) error
	TakeAttempt() (version string, ok bool)
	SaveRestore(entities.RestoreTabs) error
	TakeRestore() (entities.RestoreTabs, bool)
	Cleanup()
}

// StateSink is called with the usecase lock held: it must not call back into the usecase.
type StateSink interface {
	Publish(entities.UpdateState)
}

type usecase struct {
	current  string
	platform Platform
	src      ManifestSource
	codec    ManifestCodec
	dl       Downloader
	inst     Installer
	store    Store
	sink     StateSink

	mu       sync.Mutex
	state    entities.UpdateState
	release  *entities.Release
	raw      []byte
	artifact entities.Artifact
	cancel   context.CancelFunc
}

func NewUsecase(current string, p Platform, src ManifestSource, codec ManifestCodec, dl Downloader,
	inst Installer, store Store, sink StateSink) Usecase {
	return &usecase{
		current:  current,
		platform: p,
		src:      src,
		codec:    codec,
		dl:       dl,
		inst:     inst,
		store:    store,
		sink:     sink,
		state:    entities.UpdateState{Phase: entities.UpdateIdle, Current: current},
	}
}

func (u *usecase) Status() entities.UpdateState {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

func (u *usecase) TakeRestore() (entities.RestoreTabs, bool) {
	return u.store.TakeRestore()
}

func (u *usecase) set(st entities.UpdateState) {
	u.state = st
	u.sink.Publish(st)
}

func (u *usecase) busy() bool {
	return u.state.Phase == entities.UpdateDownloading || u.state.Phase == entities.UpdateApplying
}

func (u *usecase) releaseState(phase entities.UpdatePhase, reason string) entities.UpdateState {
	return entities.UpdateState{
		Phase:   phase,
		Version: u.release.Version,
		Current: u.current,
		Install: entities.InstallInApp,
		Reason:  reason,
	}
}
