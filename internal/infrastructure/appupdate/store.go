package appupdate

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

const (
	manifestFile = "manifest.json"
	attemptFile  = "attempt"
	restoreFile  = "restore.json"
	restoreTTL   = 10 * time.Minute
)

type FileStore struct{ Dir string }

func NewFileStore(dataDir string) (*FileStore, error) {
	const funcName = "appupdate.NewFileStore"
	dir := filepath.Join(dataDir, "updates")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return &FileStore{Dir: dir}, nil
}

func (s *FileStore) StagePath(r entities.Release, a entities.Artifact) string {
	name := a.URL
	if u, err := url.Parse(a.URL); err == nil {
		name = u.Path
	}
	return filepath.Join(s.Dir, r.Version, path.Base(name))
}

func (s *FileStore) SaveStaged(r entities.Release, manifest []byte, _ string) error {
	const funcName = "appupdate.FileStore.SaveStaged"
	if err := writeAtomic(filepath.Join(s.Dir, r.Version, manifestFile), manifest); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (s *FileStore) Staged() ([]byte, string, bool) {
	var best, file string
	for _, v := range s.versionDirs() {
		if best != "" && !appupdate.Newer(v, best) {
			continue
		}
		if f, ok := s.artifactIn(v); ok {
			best, file = v, f
		}
	}
	if best == "" {
		return nil, "", false
	}
	manifest, err := os.ReadFile(filepath.Join(s.Dir, best, manifestFile))
	if err != nil {
		return nil, "", false
	}
	return manifest, file, true
}

func (s *FileStore) artifactIn(version string) (string, bool) {
	dir := filepath.Join(s.Dir, version)
	if _, err := os.Stat(filepath.Join(dir, manifestFile)); err != nil {
		return "", false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		name := e.Name()
		if e.Type().IsRegular() && name != manifestFile && !strings.HasSuffix(name, ".part") {
			return filepath.Join(dir, name), true
		}
	}
	return "", false
}

func (s *FileStore) DropStaged() error {
	const funcName = "appupdate.FileStore.DropStaged"
	for _, v := range s.versionDirs() {
		if err := os.RemoveAll(filepath.Join(s.Dir, v)); err != nil {
			return fmt.Errorf("%s: %w", funcName, err)
		}
	}
	return nil
}

func (s *FileStore) versionDirs() []string {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && appupdate.ValidVersion(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

func (s *FileStore) MarkAttempt(version string) error {
	const funcName = "appupdate.FileStore.MarkAttempt"
	if err := writeAtomic(filepath.Join(s.Dir, attemptFile), []byte(version)); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (s *FileStore) TakeAttempt() (string, bool) {
	b, ok := s.take(attemptFile)
	return string(b), ok
}

func (s *FileStore) SaveRestore(r entities.RestoreTabs) error {
	const funcName = "appupdate.FileStore.SaveRestore"
	dto := restoreDTO{
		WorkspaceID: r.WorkspaceID,
		Tabs:        make([]restoreTabDTO, 0, len(r.Tabs)),
		ActiveTabID: r.ActiveTabID,
		SavedAt:     r.SavedAt,
	}
	for _, t := range r.Tabs {
		dto.Tabs = append(dto.Tabs, restoreTabDTO{Type: t.Type, ID: t.ID})
	}
	b, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if err := writeAtomic(filepath.Join(s.Dir, restoreFile), b); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (s *FileStore) TakeRestore() (entities.RestoreTabs, bool) {
	b, ok := s.take(restoreFile)
	if !ok {
		return entities.RestoreTabs{}, false
	}
	var dto restoreDTO
	if err := json.Unmarshal(b, &dto); err != nil || time.Since(dto.SavedAt) > restoreTTL {
		return entities.RestoreTabs{}, false
	}
	r := entities.RestoreTabs{WorkspaceID: dto.WorkspaceID, ActiveTabID: dto.ActiveTabID, SavedAt: dto.SavedAt}
	for _, t := range dto.Tabs {
		r.Tabs = append(r.Tabs, entities.RestoreTab{Type: t.Type, ID: t.ID})
	}
	return r, true
}

func (s *FileStore) take(name string) ([]byte, bool) {
	p := filepath.Join(s.Dir, name)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	if err := os.Remove(p); err != nil {
		slog.Warn("appupdate: failed to remove a one-shot file", "file", name, "err", err)
	}
	return b, true
}

func (s *FileStore) Cleanup() {
	for _, pattern := range []string{"*.part", filepath.Join("*", "*.part")} {
		parts, _ := filepath.Glob(filepath.Join(s.Dir, pattern))
		for _, p := range parts {
			_ = os.Remove(p)
		}
	}
}

func writeAtomic(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+"-*.part")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), name)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
}
