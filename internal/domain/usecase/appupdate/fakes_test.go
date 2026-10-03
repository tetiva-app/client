package appupdate

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type fakeSource struct {
	raw   []byte
	err   error
	calls int
}

func (s *fakeSource) Fetch(context.Context) ([]byte, error) {
	s.calls++
	return s.raw, s.err
}

type fakeCodec struct {
	rel entities.Release
	err error
}

func (c *fakeCodec) Parse([]byte) (entities.Release, error) {
	return c.rel, c.err
}

type fakeDownloader struct {
	data    []byte
	err     error
	calls   int
	block   chan struct{}
	started chan struct{}
}

func (d *fakeDownloader) Download(ctx context.Context, _ entities.Artifact, dst string, progress func(int64)) error {
	d.calls++
	if err := os.WriteFile(dst, d.data, 0o600); err != nil {
		return err
	}
	progress(int64(len(d.data)))
	if d.started != nil {
		d.started <- struct{}{}
	}
	if d.block != nil {
		select {
		case <-d.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return d.err
}

type fakeInstaller struct {
	kind      entities.InstallKind
	reason    string
	verifyErr error
	applyErr  error
	verified  int
	applied   int
}

func (i *fakeInstaller) Kind(context.Context) (entities.InstallKind, string) {
	return i.kind, i.reason
}

func (i *fakeInstaller) Verify(context.Context, entities.Release, string) error {
	i.verified++
	return i.verifyErr
}

func (i *fakeInstaller) Apply(context.Context, entities.Release, string) error {
	i.applied++
	return i.applyErr
}

type memStore struct {
	dir        string
	manifest   []byte
	file       string
	staged     bool
	saveCalls  int
	attempt    string
	hasAttempt bool
	restore    *entities.RestoreTabs
}

func (s *memStore) StagePath(r entities.Release, a entities.Artifact) string {
	return filepath.Join(s.dir, r.Version, path.Base(a.URL))
}

func (s *memStore) SaveStaged(_ entities.Release, manifest []byte, file string) error {
	s.saveCalls++
	s.manifest, s.file, s.staged = manifest, file, true
	return nil
}

func (s *memStore) Staged() ([]byte, string, bool) {
	return s.manifest, s.file, s.staged
}

func (s *memStore) DropStaged() error {
	if s.staged {
		if err := os.RemoveAll(filepath.Dir(s.file)); err != nil {
			return err
		}
	}
	s.manifest, s.file, s.staged = nil, "", false
	return nil
}

func (s *memStore) MarkAttempt(version string) error {
	s.attempt, s.hasAttempt = version, true
	return nil
}

func (s *memStore) TakeAttempt() (string, bool) {
	v, ok := s.attempt, s.hasAttempt
	s.attempt, s.hasAttempt = "", false
	return v, ok
}

func (s *memStore) SaveRestore(r entities.RestoreTabs) error {
	s.restore = &r
	return nil
}

func (s *memStore) TakeRestore() (entities.RestoreTabs, bool) {
	if s.restore == nil {
		return entities.RestoreTabs{}, false
	}
	r := *s.restore
	s.restore = nil
	return r, true
}

func (s *memStore) Cleanup() {}

type recSink struct {
	mu     sync.Mutex
	states []entities.UpdateState
}

func (s *recSink) Publish(st entities.UpdateState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states = append(s.states, st)
}

func (s *recSink) phases() []entities.UpdatePhase {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]entities.UpdatePhase, len(s.states))
	for i, st := range s.states {
		out[i] = st.Phase
	}
	return out
}
