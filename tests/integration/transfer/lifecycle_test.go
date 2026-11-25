//go:build integration

package transfer_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"sync/atomic"
	"testing"
	"time"
)

type slowWriter struct {
	http.ResponseWriter
	ctx context.Context
}

func TestNonResumableServerRequiresFreshDownloadAndPreservesPartial(t *testing.T) {
	for _, behavior := range []string{"ignore-range", "wrong-content-range"} {
		t.Run(behavior, func(t *testing.T) {
			binary, err := aria2.FindBinary()
			if err != nil {
				t.Fatal(err)
			}
			data := bytes.Repeat([]byte("non-resumable-file\n"), 100000)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", fmt.Sprint(len(data)))
				if r.Header.Get("Range") != "" && behavior == "wrong-content-range" {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)))
					w.WriteHeader(http.StatusPartialContent)
				}
				_, _ = (slowWriter{w, r.Context()}).Write(data)
			}))
			defer server.Close()
			stateDir, target := t.TempDir(), t.TempDir()
			proc, err := aria2.Start(context.Background(), stateDir, binary)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := proc.Close(ctx); err != nil {
					t.Error(err)
				}
			}()
			n := 0
			svc := application.New(proc.Client, persistence.Repository{Path: filepath.Join(stateDir, "tasks.json")}, target, func() string { n++; return fmt.Sprintf("%016x", n) })
			if err := svc.Open(context.Background()); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Add(context.Background(), server.URL+"/fixture.bin", target); err != nil {
				t.Fatal(err)
			}
			id := svc.Snapshot().Tasks[0].ID
			awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Status == "active" && task.Completed > 0 })
			if err := svc.Execute(context.Background(), id, application.Pause); err != nil {
				t.Fatal(err)
			}
			if err := svc.Execute(context.Background(), id, application.Resume); err != nil {
				t.Fatal(err)
			}
			failed := awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Status == "error" })
			if !failed.CanRestart || failed.CanRetry || failed.Failure != application.FailureResumeUnsupported {
				t.Fatalf("generic/unactionable failure: %+v", failed)
			}
			original, err := os.ReadFile(failed.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Execute(context.Background(), id, application.Restart); err != nil {
				t.Fatal(err)
			}
			done := awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Status == "complete" })
			if done.Path == failed.Path {
				t.Fatal("fresh download overwrote original partial")
			}
			got, err := os.ReadFile(done.Path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, data) {
				t.Fatal("fresh download is corrupt")
			}
			preserved, err := os.ReadFile(failed.Path)
			if err != nil || !bytes.Equal(preserved, original) {
				t.Fatal("original partial file was modified")
			}
		})
	}
}

func (w slowWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		n := min(len(p), 16<<10)
		select {
		case <-w.ctx.Done():
			return total, w.ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		written, err := w.ResponseWriter.Write(p[:n])
		total += written
		if f, ok := w.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
		if err != nil {
			return total, err
		}
		p = p[n:]
	}
	return total, nil
}
func awaitTask(t *testing.T, s *application.Service, id string, accept func(application.TaskDTO) bool) application.TaskDTO {
	t.Helper()
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	var last application.TaskDTO
	for {
		if err := s.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		for _, task := range s.Snapshot().Tasks {
			if task.ID == id {
				last = task
				if accept(task) {
					return task
				}
			}
		}
		select {
		case <-deadline.C:
			t.Fatalf("task did not converge: %+v", last)
		case <-ticker.C:
		}
	}
}
func TestRealDownloadPauseRestartResumeAndRemoveKeepsFile(t *testing.T) {
	binary, err := aria2.FindBinary()
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("skiff-download-fixture\n"), 100000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(slowWriter{w, r.Context()}, r, "fixture.bin", time.Unix(1700000000, 0), bytes.NewReader(data))
	}))
	defer server.Close()
	stateDir := t.TempDir()
	target := t.TempDir()
	proc, err := aria2.Start(context.Background(), stateDir, binary)
	if err != nil {
		t.Fatal(err)
	}
	closeProcess := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Close(ctx); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() {
		if proc != nil {
			closeProcess()
		}
	})
	repo := persistence.Repository{Path: filepath.Join(stateDir, "tasks.json")}
	svc := application.New(proc.Client, repo, target, func() string { return "0123456789abcdef" })
	if err := svc.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(context.Background(), server.URL+"/fixture.bin", target); err != nil {
		t.Fatal(err)
	}
	id := svc.Snapshot().Tasks[0].ID
	awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Completed > 0 && task.Status == "active" })
	if err := svc.Execute(context.Background(), id, application.Pause); err != nil {
		t.Fatal(err)
	}
	paused := svc.Snapshot().Tasks[0]
	if !paused.CanResume {
		t.Fatal("pause not reflected")
	}
	closeProcess()
	proc = nil
	proc, err = aria2.Start(context.Background(), stateDir, binary)
	if err != nil {
		t.Fatal(err)
	}
	restored := application.New(proc.Client, repo, target, func() string { return "fedcba9876543210" })
	if err := restored.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := restored.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if task := restored.Snapshot().Tasks[0]; task.ID != id || !task.CanResume || task.Total != paused.Total || task.Completed != paused.Completed {
		t.Fatalf("restore=%+v", task)
	}
	if err := restored.Execute(context.Background(), id, application.Resume); err != nil {
		t.Fatal(err)
	}
	done := awaitTask(t, restored, id, func(task application.TaskDTO) bool { return task.Status == "complete" })
	got, err := os.ReadFile(done.Path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatal("download bytes differ")
	}
	if err := restored.Execute(context.Background(), id, application.Remove); err != nil {
		t.Fatal(err)
	}
	if len(restored.Snapshot().Tasks) != 0 {
		t.Fatal("history record not removed")
	}
	if _, err := os.Stat(done.Path); err != nil {
		t.Fatal("removal deleted the downloaded file")
	}
}
func TestRealHTTPFailureCanRetrySuccessfully(t *testing.T) {
	cases := []struct {
		path, failure string
		status        int
	}{
		{"/missing", string(application.ErrDownloadNotFound), http.StatusNotFound},
		{"/gone", string(application.ErrDownloadNotFound), http.StatusGone},
		{"/unavailable", application.FailureSourceUnavailable, http.StatusServiceUnavailable},
	}
	var ready atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			for _, tc := range cases {
				if r.URL.Path == tc.path {
					http.Error(w, http.StatusText(tc.status), tc.status)
					return
				}
			}
		}
		fmt.Fprint(w, "retried successfully")
	}))
	defer server.Close()
	svc := downloadService(t)
	for _, tc := range cases {
		if _, err := svc.Add(t.Context(), server.URL+tc.path, svc.Snapshot().Directory); err != nil {
			t.Fatal(err)
		}
		tasks := svc.Snapshot().Tasks
		failed := awaitTask(t, svc, tasks[len(tasks)-1].ID, func(task application.TaskDTO) bool { return task.CanRetry })
		if failed.Failure != tc.failure || failed.Error == "" {
			t.Errorf("HTTP %d failure = %q, want %q with an explanation", tc.status, failed.Failure, tc.failure)
		}
	}
	ready.Store(true)
	for _, task := range svc.Snapshot().Tasks {
		if err := svc.Execute(t.Context(), task.ID, application.Retry); err != nil {
			t.Fatal(err)
		}
		awaitTask(t, svc, task.ID, func(task application.TaskDTO) bool { return task.Status == "complete" })
	}
}

func TestFailedHistoryWithoutEngineEntryCanRecover(t *testing.T) {
	binary, err := aria2.FindBinary()
	if err != nil {
		t.Fatal(err)
	}
	proc, err := aria2.Start(context.Background(), t.TempDir(), binary)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	body := []byte("recovered content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	for i, cmd := range []application.Command{application.Retry, application.Restart, application.Remove} {
		t.Run(string(cmd), func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			id := fmt.Sprintf("%016x", i+1)
			repo := persistence.Repository{Path: filepath.Join(t.TempDir(), "tasks.json")}
			history := application.State{Version: 1, Directory: dir, Tasks: []domain.Record{{ID: id, URL: server.URL + "/__down", Name: "__down", Directory: dir, Status: domain.Failed, Error: "下载引擎 HTTP 状态 400"}}}
			if err := repo.Save(ctx, history); err != nil {
				t.Fatal(err)
			}
			s := application.New(proc.Client, repo, dir, func() string { return "00000000000000ff" })
			if err := s.Open(ctx); err != nil {
				t.Fatal(err)
			}
			if err := s.Execute(ctx, id, cmd); err != nil {
				t.Fatal(err)
			}
			if cmd == application.Remove {
				if len(s.Snapshot().Tasks) != 0 {
					t.Fatal("failed history still present")
				}
				return
			}
			done := awaitTask(t, s, id, func(t application.TaskDTO) bool { return t.Status == "complete" })
			got, err := os.ReadFile(done.Path)
			if err != nil || !bytes.Equal(body, got) {
				t.Fatalf("download not recovered: %v", err)
			}
		})
	}
}
