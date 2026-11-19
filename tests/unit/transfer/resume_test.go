package transfer_test

import (
	"context"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
	"skiff/internal/contexts/transfer/presentation"
	"strings"
	"testing"
)

func TestUnsupportedResumeDoesNotOfferBlindRetry(t *testing.T) {
	task := domain.Restore(domain.Record{Status: domain.Failed, Failure: "resume_unsupported", Error: "No URI available."})
	if task.Allows(domain.Retry) || !task.Allows(domain.Restart) {
		t.Fatal("unsupported resume must offer an explicit fresh download instead of repeating resume")
	}
}

func TestLegacyFailureCanRestartWithoutGuessingItsCause(t *testing.T) {
	task := domain.Restore(domain.Record{Status: domain.Failed, Error: "No URI available."})
	if !task.Allows(domain.Restart) || !task.Allows(domain.Retry) {
		t.Fatal("legacy failures need a safe explicit restart without guessing their cause")
	}
}

func TestRestoredTerminalHistoryDoesNotBecomeAPausedDownload(t *testing.T) {
	for _, status := range []domain.Status{domain.Failed, domain.Complete} {
		f := &fakeEngine{observed: []application.Observation{{ID: "saved", Status: "paused"}}}
		r := &fakeRepo{state: application.State{Version: 1, Tasks: []domain.Record{{ID: "saved", URL: "https://example.com/a", Directory: "/downloads", Status: status}}}}
		s := service(t, f, r)
		if len(f.commands) != 1 || f.commands[0] != application.Remove {
			t.Fatalf("restored terminal task was left in engine: %v", f.commands)
		}
		f.observed = nil
		if err := s.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		if s.Snapshot().Tasks[0].Status != string(status) {
			t.Fatal("terminal history changed into a resumable task")
		}
	}
}
func TestResumeFailureIsExplainedAndRestartGetsANewOutput(t *testing.T) {
	f, r := &fakeEngine{}, &fakeRepo{}
	s := service(t, f, r)
	dir := s.Snapshot().Directory
	if _, err := s.Add(context.Background(), "https://example.com/archive.zip", dir); err != nil {
		t.Fatal(err)
	}
	id := s.Snapshot().Tasks[0].ID
	f.observed = []application.Observation{{ID: id, Status: "error", Failure: "resume_unsupported", Error: "No URI available.", Total: 100, Completed: 45, Path: filepath.Join(dir, "archive.zip")}}
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	task := s.Snapshot().Tasks[0]
	if !strings.Contains(task.Error, "不支持断点续传") || !task.CanRestart || task.CanRetry {
		t.Fatalf("unhelpful failure: %+v", task)
	}
	if r.state.Tasks[0].Failure != "resume_unsupported" {
		t.Fatal("failure classification not persisted")
	}
	if err := s.Execute(context.Background(), id, application.Restart); err != nil {
		t.Fatal(err)
	}
	spec := f.enqueued[len(f.enqueued)-1]
	if spec.OutputName == "" || spec.OutputName == "archive.zip" || spec.Directory != dir {
		t.Fatalf("restart would reuse original partial file: %+v", spec)
	}
	task = s.Snapshot().Tasks[0]
	if task.ID != id || task.Completed != 0 || task.Path != "" || task.Status != "waiting" {
		t.Fatalf("restart retained stale state: %+v", task)
	}
}
func TestUnsupportedResumeViewExplainsAndConfirmsRestart(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "file.zip", Status: "error", Error: "服务器不支持断点续传，请从头下载", CanRestart: true}}
	tt := ui.NewTester(v.Render, 800, 560)
	if !tt.HasText("服务器不支持断点续传，请从头下载") {
		t.Fatal("resume failure is hidden")
	}
	if err := tt.Click("从头下载"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("原来的未完成文件会保留，新下载使用不同文件名。") {
		t.Fatal("restart confirmation must explain file preservation")
	}
}

func TestRestartsKeepStableFileNameAcrossLegacyRecords(t *testing.T) {
	const legacy = "__down.restart-b3d04c91886174e7.restart-e431adf7fc711393"
	f, r := &fakeEngine{}, &fakeRepo{state: application.State{Version: 1, Tasks: []domain.Record{{ID: "a", Name: legacy, OutputName: legacy, URL: "https://example.org/__down", Directory: t.TempDir(), Status: domain.Failed}}}}
	s := service(t, f, r)
	if got := s.Snapshot().Tasks[0].Name; got != "__down" {
		t.Errorf("restored name = %q", got)
	}
	for range 2 {
		if err := s.Execute(context.Background(), "a", application.Restart); err != nil {
			t.Fatal(err)
		}
		name := f.enqueued[len(f.enqueued)-1].OutputName
		if !strings.HasPrefix(name, "__down.restart-") || strings.Count(name, ".restart-") != 1 {
			t.Errorf("nested output name %q", name)
		}
		f.observed = []application.Observation{{ID: "a", Status: "error", Name: name}}
		if err := s.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := s.Snapshot().Tasks[0].Name; got != "__down" {
			t.Errorf("engine overwrote display name: %q", got)
		}
	}
}
