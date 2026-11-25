package transfer_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
	"strings"
	"testing"
)

type fakeEngine struct {
	concurrency int
	limitErr    error
	enqueued    []application.TransferSpec
	commands    []application.Command
	observed    []application.Observation
	addErr      error
}

func (f *fakeEngine) ProtectCheckpoints(context.Context, string, []application.OutputCandidate) (map[string]string, error) {
	return nil, nil
}

func (f *fakeEngine) SetDownloadLimit(context.Context, string, int64) error { return nil }
func (f *fakeEngine) SetConcurrency(_ context.Context, limit int) error {
	if f.limitErr != nil {
		return f.limitErr
	}
	f.concurrency = limit
	return nil
}
func (f *fakeEngine) HasPartial(context.Context, domain.Record) (bool, error) { return false, nil }
func (f *fakeEngine) InspectRequest(context.Context, application.TransferSpec, bool) (application.SourceInfo, error) {
	return application.SourceInfo{SourceVersion: domain.SourceVersion{ETag: `"fixture"`}, RangeBytes: 128 << 20}, nil
}
func (f *fakeEngine) Enqueue(_ context.Context, spec application.TransferSpec) error {
	f.enqueued = append(f.enqueued, spec)
	return f.addErr
}
func (f *fakeEngine) Execute(_ context.Context, _ string, cmd application.Command) error {
	f.commands = append(f.commands, cmd)
	return nil
}
func (f *fakeEngine) Observe(context.Context) ([]application.Observation, error) {
	return f.observed, nil
}

type fakeRepo struct {
	state   application.State
	saveErr error
	saves   int
}

func (f *fakeRepo) Load(context.Context) (application.State, error) { return f.state, nil }
func (f *fakeRepo) Save(_ context.Context, s application.State) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = s
	f.saves++
	return nil
}
func service(t *testing.T, f *fakeEngine, r *fakeRepo) *application.Service {
	t.Helper()
	n := 0
	s := application.New(f, r, t.TempDir(), func() string { n++; return fmt.Sprintf("%016x", n) })
	if err := s.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBatchQueuesValidLinesAndKeepsOnlyInvalidInput(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	dir := s.Snapshot().Directory
	if _, err := s.Add(t.Context(), "https://example.com/existing", dir); err != nil {
		t.Fatal(err)
	}
	text := "https://example.com/a\nfile:///tmp/b\nhttps://example.com/a\nhttps://example.com/existing"
	result, err := s.AddBatch(t.Context(), text, dir)
	if err != nil || result.Added != 1 || result.Duplicates != 2 || result.Invalid != 1 || strings.Join(result.Remaining, "\n") != "file:///tmp/b" || len(f.enqueued) != 2 {
		t.Fatalf("batch %+v, %v", result, err)
	}
}

func TestAddCommitsIntentBeforeStartingAndReturnsIndependentSnapshot(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	chosen := t.TempDir()
	n, err := s.Add(context.Background(), "https://example.com/a.zip\nhttps://example.com/a.zip", chosen)
	if err != nil || n != 1 {
		t.Fatalf("add = %d, %v", n, err)
	}
	if len(f.enqueued) != 1 || f.enqueued[0].Paused || r.state.Directory != chosen || len(r.state.Tasks) != 1 {
		t.Fatalf("intent not committed: %+v, %+v", f, r)
	}
	snap := s.Snapshot()
	snap.Tasks[0].Name = "changed"
	if s.Snapshot().Tasks[0].Name != "a.zip" {
		t.Fatal("snapshot mutates live task")
	}
}

func TestPersistenceFailurePreventsStartingUntrackedDownload(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	r.saveErr = errors.New("disk full")
	if n, err := s.Add(context.Background(), "https://example.com/a", s.Snapshot().Directory); err == nil || n != 0 {
		t.Fatalf("add=%d, %v", n, err)
	}
	if len(f.enqueued) != 0 || len(s.Snapshot().Tasks) != 0 {
		t.Fatal("untracked download started")
	}
}

func TestAddRejectsRelativeDirectoryBeforeCreatingTasks(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	if n, err := s.Add(context.Background(), "https://example.com/a", "relative/folder"); err == nil || n != 0 {
		t.Fatalf("relative directory accepted: %d, %v", n, err)
	}
	if len(f.enqueued) != 0 || len(s.Snapshot().Tasks) != 0 {
		t.Fatal("invalid target created a task")
	}
}

func TestFailedEnqueueRemainsVisibleAndCanBeRetried(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	f.addErr = errors.New("engine rejected transfer")
	_, _ = s.Add(context.Background(), "https://example.com/a", s.Snapshot().Directory)
	tasks := s.Snapshot().Tasks
	if len(tasks) != 1 || !tasks[0].CanRetry || tasks[0].Error == "" {
		t.Fatalf("failure lost: %+v", tasks)
	}
	f.addErr = nil
	if err := s.Execute(context.Background(), tasks[0].ID, application.Retry); err != nil {
		t.Fatal(err)
	}
	if len(f.enqueued) != 2 || s.Snapshot().Tasks[0].CanRetry {
		t.Fatal("retry did not restart task")
	}
}

func TestPauseResumeAndRemoveUseCase(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	if _, err := s.Add(context.Background(), "https://example.com/a", s.Snapshot().Directory); err != nil {
		t.Fatal(err)
	}
	id := s.Snapshot().Tasks[0].ID
	for _, cmd := range []application.Command{application.Pause, application.Resume, application.Remove} {
		if err := s.Execute(context.Background(), id, cmd); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.Snapshot().Tasks) != 0 || len(f.commands) != 3 {
		t.Fatalf("remove/commands failed: %+v", f)
	}
}

func TestRestorePausesMissingTransfersAndKeepsCompletedHistory(t *testing.T) {
	f := &fakeEngine{}
	r := &fakeRepo{state: application.State{Version: 1, Directory: "/saved", Tasks: []domain.Record{
		{ID: "unfinished", URL: "https://example.com/a", Directory: "/saved", Status: domain.Active, Completed: 25, Total: 100},
		{ID: "done", URL: "https://example.com/b", Directory: "/saved", Status: domain.Complete, Completed: 100, Total: 100},
	}}}
	s := service(t, f, r)
	if len(f.enqueued) != 1 || !f.enqueued[0].Paused || f.enqueued[0].ID != "unfinished" {
		t.Fatalf("restore enqueue=%+v", f.enqueued)
	}
	snap := s.Snapshot()
	if snap.Directory != "/saved" || snap.Tasks[0].Status != "paused" || snap.Tasks[0].Completed != 25 || snap.Tasks[1].Status != "complete" {
		t.Fatalf("restore lost state: %+v", snap)
	}
}

func TestRefreshAppliesEngineProgressWithoutChangingTaskIdentity(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	dir := s.Snapshot().Directory
	if _, err := s.Add(context.Background(), "https://example.com/a", dir); err != nil {
		t.Fatal(err)
	}
	id := s.Snapshot().Tasks[0].ID
	f.observed = []application.Observation{{ID: id, Status: "active", Total: 100, Completed: 50, Speed: 10, Name: "server-name.zip", Path: filepath.Join(dir, "server-name.zip")}}
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if snap.Speed != 10 || snap.Active != 1 || snap.Tasks[0].Progress != .5 || snap.Tasks[0].Name != "server-name.zip" || snap.Tasks[0].ID != id {
		t.Fatalf("bad projection: %+v", snap)
	}
}

type partialPauseEngine struct{ fakeEngine }

func (e *partialPauseEngine) Execute(ctx context.Context, id string, command application.Command) error {
	if command == application.Pause {
		switch id {
		case "1":
			return application.ErrEngineConnection
		case "3":
			return application.ErrCommandUnavailable
		}
	}
	return e.fakeEngine.Execute(ctx, id, command)
}

func TestPauseBatchContinuesAfterAnIndividualFailure(t *testing.T) {
	next := 0
	s := application.New(&partialPauseEngine{}, &fakeRepo{}, t.TempDir(), func() string { next++; return fmt.Sprint(next) })
	if _, err := s.Add(context.Background(), "https://example.org/one.zip\nhttps://example.org/two.zip", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	err := s.SetPaused(context.Background(), []string{"1", "2"}, true)
	if err == nil || !strings.Contains(err.Error(), "one.zip") || s.Snapshot().Tasks[1].Status != "paused" {
		t.Fatalf("batch lost the failure or skipped the second task: %v, %+v", err, s.Snapshot())
	}
}

func TestPreferencesSurviveRestartAndTemporaryDestination(t *testing.T) {
	r := &fakeRepo{}
	s := service(t, &fakeEngine{}, r)
	dir := t.TempDir()
	if err := s.SetDefaultDirectory(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStaticBackground(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(context.Background(), "https://example.org/one", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	again := service(t, &fakeEngine{}, r)
	if !again.Snapshot().Preferences.StaticBackground {
		t.Fatal("motion setting lost after restart")
	}
	if got := again.Snapshot().Directory; got != dir {
		t.Fatalf("lost configured directory %q", got)
	}
}

func TestSettingsSaveFailureKeepsPreviousValues(t *testing.T) {
	r := &fakeRepo{}
	s := service(t, &fakeEngine{}, r)
	before := s.Snapshot()
	r.saveErr = errors.New("disk full")
	if err := s.SetDefaultDirectory(context.Background(), t.TempDir()); err == nil {
		t.Fatal("lost save error")
	}
	if err := s.SetStaticBackground(context.Background(), true); err == nil {
		t.Fatal("lost save error")
	}
	if err := s.SetTheme(context.Background(), application.ThemeDark); err == nil {
		t.Fatal("theme save error lost")
	}
	if got := s.Snapshot(); got.Preferences != before.Preferences || got.Directory != before.Directory {
		t.Fatal("failed settings applied")
	}
}

func TestQueuePreferenceSurvivesRestartAndEngineRetry(t *testing.T) {
	f, repo := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, repo)
	if f.concurrency != 10 {
		t.Fatalf("old preferences should default to ten: %d", f.concurrency)
	}
	for _, bad := range []int{0, 201} {
		if err := s.SetConcurrency(t.Context(), bad); err == nil {
			t.Fatal("invalid concurrency accepted", bad)
		}
	}
	repo.saveErr = errors.New("disk full")
	if err := s.SetConcurrency(t.Context(), 1); err == nil || f.concurrency != 10 {
		t.Fatal("uncommitted limit applied")
	}
	repo.saveErr = nil
	f.limitErr = errors.New("offline")
	if err := s.SetConcurrency(t.Context(), 2); err == nil || repo.state.Preferences.Concurrency != 2 {
		t.Fatal("pending limit lost")
	}
	f.limitErr = nil
	if err := s.Refresh(t.Context()); err != nil || f.concurrency != 2 {
		t.Fatal("limit was not reconciled", err)
	}
	restored := &fakeEngine{}
	_ = service(t, restored, repo)
	if restored.concurrency != 2 {
		t.Fatal("restart did not configure the engine")
	}
}

func TestMirrorFallbackRetainsSpecificSourceFailures(t *testing.T) {
	for _, failure := range []string{string(application.ErrDownloadNotFound), string(application.ErrTooManyRedirects), string(application.ErrDownloadAuthorization), string(application.ErrDownloadChecksum)} {
		t.Run(failure, func(t *testing.T) {
			engine := &fakeEngine{}
			svc := service(t, engine, &fakeRepo{})
			const backup = "https://mirror.test/backup.zip"
			_, err := svc.AddWithOptions(t.Context(), "https://primary.test/file.zip", t.TempDir(), application.AddOptions{Mirrors: backup})
			if err != nil {
				t.Fatal(err)
			}
			id := svc.Snapshot().Tasks[0].ID
			engine.observed = []application.Observation{{ID: id, Status: "error", Failure: failure}}
			if err := svc.Refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(engine.enqueued) != 2 || engine.enqueued[1].URL != backup {
				t.Fatalf("source failure did not advance the mirror: %+v", engine.enqueued)
			}
		})
	}
}
