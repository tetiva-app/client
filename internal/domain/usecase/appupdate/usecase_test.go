package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

var (
	payload = []byte("tetiva universal zip")
	mac     = Platform{OS: "darwin", Arch: "arm64"}
)

func checksum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func testRelease(version string) entities.Release {
	return entities.Release{
		Version: version,
		Artifacts: []entities.Artifact{
			{OS: "darwin", Arch: "universal", Format: "zip", URL: "https://s3.example/Tetiva-" + version + "-macos-universal.zip", SHA256: checksum(payload), Size: int64(len(payload))},
			{OS: "windows", Arch: "amd64", Format: "nsis", URL: "https://s3.example/Tetiva-" + version + "-windows-amd64-installer.exe", SHA256: checksum(payload), Size: int64(len(payload))},
		},
	}
}

type harness struct {
	uc    Usecase
	src   *fakeSource
	codec *fakeCodec
	dl    *fakeDownloader
	inst  *fakeInstaller
	store *memStore
	sink  *recSink
}

func newHarness(t *testing.T, p Platform, rel entities.Release) *harness {
	t.Helper()
	h := &harness{
		src:   &fakeSource{raw: []byte("manifest " + rel.Version)},
		codec: &fakeCodec{rel: rel},
		dl:    &fakeDownloader{data: payload},
		inst:  &fakeInstaller{kind: entities.InstallInApp},
		store: &memStore{dir: t.TempDir()},
		sink:  &recSink{},
	}
	h.uc = NewUsecase("1.2.1", p, h.src, h.codec, h.dl, h.inst, h.store, h.sink)
	return h
}

func (h *harness) stage(t *testing.T, rel entities.Release, data []byte) string {
	t.Helper()
	file := h.store.StagePath(rel, rel.Artifacts[0])
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.store.SaveStaged(rel, []byte("manifest "+rel.Version), file); err != nil {
		t.Fatal(err)
	}
	h.store.saveCalls = 0
	return file
}

func (h *harness) reachReady(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	h.uc.Check(ctx)
	if st := h.uc.Download(ctx); st.Phase != entities.UpdateReady {
		t.Fatalf("download: phase %q reason %q, want ready", st.Phase, st.Reason)
	}
}

func assertPhase(t *testing.T, st entities.UpdateState, phase entities.UpdatePhase, reason string) {
	t.Helper()
	if st.Phase != phase || st.Reason != reason {
		t.Fatalf("state = %q/%q, want %q/%q", st.Phase, st.Reason, phase, reason)
	}
}

func assertGone(t *testing.T, file string) {
	t.Helper()
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s still exists (err %v)", file, err)
	}
}

func TestCheck_NewerInApp(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	st := h.uc.Check(context.Background())
	assertPhase(t, st, entities.UpdateAvailable, "")
	if st.Version != "1.2.2" || st.Install != entities.InstallInApp || st.Current != "1.2.1" {
		t.Fatalf("state = %+v", st)
	}
}

func TestCheck_SameVersion_UpToDate_DropsStaged(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.1"))
	file := h.stage(t, testRelease("1.2.2"), payload)
	assertPhase(t, h.uc.Check(context.Background()), entities.UpdateUpToDate, "")
	if h.store.staged {
		t.Fatal("staged copy kept")
	}
	assertGone(t, file)
}

func TestCheck_BadManifest(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.codec.err = errors.New("no valid signature")
	assertPhase(t, h.uc.Check(context.Background()), entities.UpdateError, ReasonBadManifest)
	if h.dl.calls != 0 {
		t.Fatalf("downloader called %d times", h.dl.calls)
	}
}

func TestCheck_Network(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.src.err = errors.New("dial tcp: no route to host")
	assertPhase(t, h.uc.Check(context.Background()), entities.UpdateError, ReasonNetwork)
}

func TestCheck_NetworkWhileReady_StaysReady(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.reachReady(t)
	h.src.err = errors.New("offline")
	seen := len(h.sink.phases())

	assertPhase(t, h.uc.Check(context.Background()), entities.UpdateReady, "")
	if slices.Contains(h.sink.phases()[seen:], entities.UpdateChecking) {
		t.Fatalf("published checking while ready: %v", h.sink.phases())
	}
}

func TestCheck_LinuxAPT(t *testing.T) {
	h := newHarness(t, Platform{OS: "linux", Arch: "amd64"}, entities.Release{Version: "1.2.2"})
	h.inst.kind = entities.InstallAPT
	st := h.uc.Check(context.Background())
	assertPhase(t, st, entities.UpdateAvailable, "")
	if st.Install != entities.InstallAPT {
		t.Fatalf("install = %q, want apt", st.Install)
	}
}

func TestCheck_NoArtifact(t *testing.T) {
	h := newHarness(t, Platform{OS: "windows", Arch: "arm64"}, testRelease("1.2.2"))
	st := h.uc.Check(context.Background())
	assertPhase(t, st, entities.UpdateAvailable, ReasonNoArtifact)
	if st.Install != entities.InstallUnsupported {
		t.Fatalf("install = %q, want unsupported", st.Install)
	}
}

func TestCheck_InAppDisabled(t *testing.T) {
	rel := testRelease("1.2.2")
	rel.InAppDisabled = []string{"1.2.1"}
	h := newHarness(t, mac, rel)
	file := h.stage(t, rel, payload)

	st := h.uc.Check(context.Background())
	assertPhase(t, st, entities.UpdateAvailable, ReasonDisabled)
	if st.Install != entities.InstallUnsupported {
		t.Fatalf("install = %q, want unsupported", st.Install)
	}
	if h.store.staged {
		t.Fatal("staged copy kept")
	}
	assertGone(t, file)
}

func TestCheck_InAppDisabled_APTKeepsKind(t *testing.T) {
	h := newHarness(t, Platform{OS: "linux", Arch: "amd64"}, entities.Release{Version: "1.2.2", InAppDisabled: []string{"1.2.1"}})
	h.inst.kind = entities.InstallAPT
	st := h.uc.Check(context.Background())
	assertPhase(t, st, entities.UpdateAvailable, "")
	if st.Install != entities.InstallAPT {
		t.Fatalf("install = %q, want apt", st.Install)
	}
}

func TestCheck_UnsupportedKind(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.inst.kind, h.inst.reason = entities.InstallUnsupported, ReasonTranslocated
	st := h.uc.Check(context.Background())
	assertPhase(t, st, entities.UpdateAvailable, ReasonTranslocated)
	if st.Install != entities.InstallUnsupported {
		t.Fatalf("install = %q, want unsupported", st.Install)
	}
}

func TestCheck_AlreadyStaged_Ready(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.stage(t, rel, payload)
	assertPhase(t, h.uc.Check(context.Background()), entities.UpdateReady, "")
	if h.dl.calls != 0 {
		t.Fatalf("downloader called %d times", h.dl.calls)
	}
}

func TestCheck_StagedOtherVersion_Dropped(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	file := h.stage(t, testRelease("1.2.3"), payload)
	assertPhase(t, h.uc.Check(context.Background()), entities.UpdateAvailable, "")
	if h.store.staged {
		t.Fatal("staged copy kept")
	}
	assertGone(t, file)
}

func TestDownload_Ready(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.reachReady(t)

	if h.dl.calls != 1 || h.inst.verified != 1 || h.store.saveCalls != 1 {
		t.Fatalf("download %d, verify %d, save %d; want 1 each", h.dl.calls, h.inst.verified, h.store.saveCalls)
	}
	file := h.store.StagePath(rel, rel.Artifacts[0])
	if h.store.file != file {
		t.Fatalf("staged file %q, want %q", h.store.file, file)
	}
	assertGone(t, file+".part")
	phases := h.sink.phases()
	d := slices.Index(phases, entities.UpdateDownloading)
	r := slices.Index(phases, entities.UpdateReady)
	if d < 0 || r < d {
		t.Fatalf("phases = %v, want downloading then ready", phases)
	}
}

func TestDownload_ChecksumMismatch(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.dl.data = []byte("tampered zip bytes!!")
	ctx := context.Background()
	h.uc.Check(ctx)

	assertPhase(t, h.uc.Download(ctx), entities.UpdateError, ReasonChecksum)
	file := h.store.StagePath(rel, rel.Artifacts[0])
	assertGone(t, file)
	assertGone(t, file+".part")
	if h.store.saveCalls != 0 {
		t.Fatal("tampered file staged")
	}
}

func TestDownload_VerifyFails(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.inst.verifyErr = &domain.ReasonError{Reason: ReasonCodesignFailed, Err: errors.New("codesign: invalid signature")}
	ctx := context.Background()
	h.uc.Check(ctx)
	assertPhase(t, h.uc.Download(ctx), entities.UpdateError, ReasonCodesignFailed)
	if h.store.saveCalls != 0 {
		t.Fatal("unverified file staged")
	}
}

func TestDownload_NotAvailable(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.1"))
	ctx := context.Background()
	before := h.uc.Check(ctx)
	if st := h.uc.Download(ctx); st != before {
		t.Fatalf("state = %+v, want %+v", st, before)
	}
	if h.dl.calls != 0 {
		t.Fatalf("downloader called %d times", h.dl.calls)
	}
}

func TestDownload_SecondCallWhileDownloading(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.dl.block, h.dl.started = make(chan struct{}), make(chan struct{}, 1)
	ctx := context.Background()
	h.uc.Check(ctx)

	done := make(chan entities.UpdateState, 1)
	go func() { done <- h.uc.Download(ctx) }()
	<-h.dl.started

	assertPhase(t, h.uc.Download(ctx), entities.UpdateDownloading, "")
	close(h.dl.block)
	assertPhase(t, <-done, entities.UpdateReady, "")
	if h.dl.calls != 1 {
		t.Fatalf("downloader called %d times", h.dl.calls)
	}
}

func TestCancel(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.dl.block, h.dl.started = make(chan struct{}), make(chan struct{}, 1)
	ctx := context.Background()
	h.uc.Check(ctx)

	done := make(chan entities.UpdateState, 1)
	go func() { done <- h.uc.Download(ctx) }()
	<-h.dl.started
	h.uc.Cancel()

	assertPhase(t, <-done, entities.UpdateAvailable, "")
	assertGone(t, h.store.StagePath(rel, rel.Artifacts[0])+".part")
}

func TestApply_OnlyFromReady(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	ctx := context.Background()
	h.uc.Check(ctx)
	if err := h.uc.Apply(ctx, entities.RestoreTabs{}); err == nil {
		t.Fatal("Apply from available succeeded")
	}
	if h.inst.applied != 0 {
		t.Fatal("installer called")
	}
}

func TestApply_Success(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.reachReady(t)
	restore := entities.RestoreTabs{
		WorkspaceID: "ws-1",
		Tabs:        []entities.RestoreTab{{Type: "request", ID: "r-1"}},
		ActiveTabID: "r-1",
	}

	if err := h.uc.Apply(context.Background(), restore); err != nil {
		t.Fatal(err)
	}
	if h.store.attempt != "1.2.2" {
		t.Fatalf("attempt = %q, want 1.2.2", h.store.attempt)
	}
	if h.store.restore == nil || h.store.restore.ActiveTabID != "r-1" || len(h.store.restore.Tabs) != 1 {
		t.Fatalf("restore = %+v", h.store.restore)
	}
	if h.inst.applied != 1 {
		t.Fatalf("installer applied %d times", h.inst.applied)
	}
	assertPhase(t, h.uc.Status(), entities.UpdateApplying, "")
}

func TestApply_Twice(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.reachReady(t)
	ctx := context.Background()
	if err := h.uc.Apply(ctx, entities.RestoreTabs{}); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Apply(ctx, entities.RestoreTabs{}); err == nil {
		t.Fatal("second Apply succeeded")
	}
	if h.inst.applied != 1 {
		t.Fatalf("installer applied %d times", h.inst.applied)
	}
}

func TestApply_Failure(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.inst.applyErr = errors.New("rename swap: operation not permitted")
	h.reachReady(t)

	err := h.uc.Apply(context.Background(), entities.RestoreTabs{WorkspaceID: "ws-1"})
	var re *domain.ReasonError
	if !errors.As(err, &re) || re.Reason != ReasonInstallFailed {
		t.Fatalf("err = %v, want ReasonError %s", err, ReasonInstallFailed)
	}
	assertPhase(t, h.uc.Status(), entities.UpdateError, ReasonInstallFailed)
	if _, ok := h.uc.TakeRestore(); ok {
		t.Fatal("restore kept after a failed install")
	}
	if _, ok := h.store.TakeAttempt(); ok {
		t.Fatal("attempt kept after a failed install")
	}
}

func TestApply_DisabledAfterDownload(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.reachReady(t)
	h.codec.rel.InAppDisabled = []string{"1.2.1"}

	if err := h.uc.Apply(context.Background(), entities.RestoreTabs{}); err == nil {
		t.Fatal("Apply of a disabled update succeeded")
	}
	assertPhase(t, h.uc.Status(), entities.UpdateAvailable, ReasonDisabled)
	if h.inst.applied != 0 {
		t.Fatal("installer called")
	}
	assertGone(t, h.store.StagePath(rel, rel.Artifacts[0]))
}

func TestApply_Offline(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	h.reachReady(t)
	h.src.err = errors.New("offline")

	if err := h.uc.Apply(context.Background(), entities.RestoreTabs{}); err != nil {
		t.Fatal(err)
	}
	if h.inst.applied != 1 {
		t.Fatalf("installer applied %d times", h.inst.applied)
	}
}

func TestApply_TamperedFile_Offline(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.reachReady(t)
	h.src.err = errors.New("offline")
	if err := os.WriteFile(h.store.StagePath(rel, rel.Artifacts[0]), []byte("tampered zip bytes!!"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.uc.Apply(context.Background(), entities.RestoreTabs{}); err == nil {
		t.Fatal("Apply of a tampered file succeeded")
	}
	assertPhase(t, h.uc.Status(), entities.UpdateError, ReasonChecksum)
	if h.inst.applied != 0 {
		t.Fatal("installer called")
	}
}

func TestStart_ReadyOffline(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.stage(t, rel, payload)

	h.uc.Start(context.Background())
	st := h.uc.Status()
	assertPhase(t, st, entities.UpdateReady, "")
	if st.Version != "1.2.2" {
		t.Fatalf("version = %q, want 1.2.2", st.Version)
	}
	if h.src.calls != 0 {
		t.Fatalf("manifest fetched %d times", h.src.calls)
	}
}

func TestStart_TamperedStaged(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	file := h.stage(t, rel, []byte("tampered zip bytes!!"))

	h.uc.Start(context.Background())
	assertPhase(t, h.uc.Status(), entities.UpdateIdle, "")
	if h.store.staged {
		t.Fatal("tampered copy kept")
	}
	assertGone(t, file)
}

func TestStart_StagedNotNewer_Dropped(t *testing.T) {
	rel := testRelease("1.2.1")
	h := newHarness(t, mac, rel)
	file := h.stage(t, rel, payload)

	h.uc.Start(context.Background())
	assertPhase(t, h.uc.Status(), entities.UpdateIdle, "")
	if h.store.staged {
		t.Fatal("staged copy kept")
	}
	assertGone(t, file)
}

func TestStart_SucceededAttempt(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.1"))
	if err := h.store.MarkAttempt("1.2.1"); err != nil {
		t.Fatal(err)
	}
	h.uc.Start(context.Background())
	assertPhase(t, h.uc.Status(), entities.UpdateIdle, "")
	if h.store.hasAttempt {
		t.Fatal("attempt not consumed")
	}
}

func TestStart_FailedAttempt_StagedGood(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.stage(t, rel, payload)
	if err := h.store.MarkAttempt("1.2.2"); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	h.uc.Start(ctx)
	assertPhase(t, h.uc.Status(), entities.UpdateReady, ReasonInstallFailed)
	if err := h.uc.Apply(ctx, entities.RestoreTabs{}); err != nil {
		t.Fatal(err)
	}
	if h.inst.applied != 1 {
		t.Fatalf("installer applied %d times", h.inst.applied)
	}
}

func TestStart_FailedAttempt_CheckKeepsReason(t *testing.T) {
	rel := testRelease("1.2.2")
	h := newHarness(t, mac, rel)
	h.stage(t, rel, payload)
	if err := h.store.MarkAttempt("1.2.2"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	h.uc.Start(ctx)

	assertPhase(t, h.uc.Check(ctx), entities.UpdateReady, ReasonInstallFailed)
}

func TestStart_FailedAttempt_NothingStaged(t *testing.T) {
	h := newHarness(t, mac, testRelease("1.2.2"))
	if err := h.store.MarkAttempt("1.2.2"); err != nil {
		t.Fatal(err)
	}
	h.uc.Start(context.Background())
	st := h.uc.Status()
	assertPhase(t, st, entities.UpdateError, ReasonInstallFailed)
	if st.Version != "1.2.2" {
		t.Fatalf("version = %q, want 1.2.2", st.Version)
	}
}
