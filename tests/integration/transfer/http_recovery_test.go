//go:build integration

package transfer_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
)

func downloadService(t *testing.T) *application.Service {
	t.Helper()
	binary, err := aria2.FindBinary()
	if err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	proc, err := aria2.Start(t.Context(), state, binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	var counter int
	service := application.New(proc.Client, persistence.Repository{Path: filepath.Join(state, "tasks.json")}, t.TempDir(), func() string { counter++; return fmt.Sprintf("%016x", counter) })
	if err := service.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestRedirectedSourceChangesBeforeResume(t *testing.T) {
	var changed atomic.Bool
	first := bytes.Repeat([]byte("a"), 3<<20)
	second := bytes.Repeat([]byte("b"), len(first))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/link" {
			http.Redirect(w, r, "/object", http.StatusFound)
			return
		}
		data, tag := first, `"v1"`
		if changed.Load() {
			data, tag = second, `"v2"`
		}
		w.Header().Set("ETag", tag)
		w.Header().Set("Content-Disposition", `attachment; filename="archive.bin"`)
		http.ServeContent(slowWriter{w, r.Context()}, r, "object", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	s := downloadService(t)
	if _, err := s.Add(t.Context(), server.URL+"/link", s.Snapshot().Directory); err != nil {
		t.Fatal(err)
	}
	id := s.Snapshot().Tasks[0].ID
	active := awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Completed > 0 })
	if active.Name != "archive.bin" {
		t.Fatalf("redirected filename %q", active.Name)
	}
	if err := s.Execute(t.Context(), id, application.Pause); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Status == "paused" })
	before, err := os.ReadFile(active.Path)
	if err != nil {
		t.Fatal(err)
	}
	changed.Store(true)
	if err := s.Execute(t.Context(), id, application.Resume); err != nil {
		t.Fatal(err)
	}
	failed := awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Status == "error" || task.Status == "complete" })
	if !failed.CanRestart || failed.CanRetry || failed.Failure != application.FailureSourceChanged {
		t.Fatalf("unsafe resume: %+v", failed)
	}
	after, err := os.ReadFile(active.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("changed source modified original")
	}
	if err := s.Execute(t.Context(), id, application.Restart); err != nil {
		t.Fatal(err)
	}
	done := awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Status == "complete" })
	got, err := os.ReadFile(done.Path)
	if err != nil || done.Path == active.Path || !bytes.Equal(second, got) {
		t.Fatal("fresh copy lost")
	}
}

func TestRecentLastModifiedCannotAuthenticateSameSecondReplacement(t *testing.T) {
	now := time.Now().UTC().Format(http.TimeFormat)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", now)
		w.Header().Set("Last-Modified", now)
		w.Header().Set("ETag", `W/"weak"`)
	}))
	defer server.Close()
	version, err := aria2.NewClient("", "").InspectSource(t.Context(), server.URL, false)
	if err != nil || version.Verified() {
		t.Fatalf("ambiguous timestamp accepted: %+v, %v", version, err)
	}
}

func TestNewTaskNeverAdoptsRemovedTasksCheckpoint(t *testing.T) {
	old := bytes.Repeat([]byte("A"), 6<<20)
	fresh := bytes.Repeat([]byte("B"), len(old))
	var changed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, etag := old, `"original"`
		if changed.Load() {
			data, etag = fresh, `"replaced"`
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Disposition", `attachment; filename="named.bin"`)
		http.ServeContent(slowWriter{w, r.Context()}, r, "link", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	svc := downloadService(t)
	dir := svc.Snapshot().Directory
	add := func() string {
		t.Helper()
		if _, err := svc.Add(t.Context(), server.URL+"/link", dir); err != nil {
			t.Fatal(err)
		}
		return svc.Snapshot().Tasks[0].ID
	}
	id := add()
	partial := awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Completed >= 1<<20 })
	if err := svc.Execute(t.Context(), id, application.Pause); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Status == "paused" })
	if err := svc.Execute(t.Context(), id, application.Remove); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(partial.Path)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := os.ReadFile(partial.Path + ".aria2")
	if err != nil {
		t.Fatal(err)
	}
	changed.Store(true)
	done := awaitTask(t, svc, add(), func(task application.TaskDTO) bool { return task.Status == "complete" })
	after, err := os.ReadFile(partial.Path)
	if err != nil || !bytes.Equal(after, before) || done.Path == partial.Path {
		t.Fatal("a new task adopted the previous partial file")
	}
	control, err := os.ReadFile(partial.Path + ".aria2")
	if err != nil || !bytes.Equal(control, checkpoint) {
		t.Fatal("old checkpoint changed")
	}
	got, err := os.ReadFile(done.Path)
	if err != nil || !bytes.Equal(got, fresh) {
		t.Fatal("mixed file versions")
	}
}

func TestUnavailableHTTPMetadataAllowsSafeFreshStart(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusGone, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			info, err := aria2.NewClient("", "").InspectSource(t.Context(), server.URL, false)
			if info.Verified() || (err != nil) != (status == http.StatusServiceUnavailable) {
				t.Fatalf("HTTP %d recovery: %+v, %v", status, info, err)
			}
		})
	}
}
