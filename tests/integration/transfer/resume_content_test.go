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
	"skiff/internal/contexts/transfer/domain"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"skiff/internal/contexts/transfer/application"
)

// The first request after clicking Continue should fetch missing file data.
func TestContinueRequestsOnlyMissingData(t *testing.T) {
	for _, tag := range []string{"", `"v1"`} {
		t.Run("etag="+tag, func(t *testing.T) {
			data := bytes.Repeat([]byte("download"), 512<<10)
			var resumed atomic.Bool
			var resumedRanges atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tag != "" {
					w.Header().Set("ETag", tag)
				}
				if resumed.Load() {
					span := r.Header.Get("Range")
					var offset int64
					_, err := fmt.Sscanf(span, "bytes=%d-", &offset)
					if r.Method != "GET" || err != nil || offset < 512<<10 {
						t.Errorf("continue requested %s %q before fetching missing data", r.Method, span)
					}
					if r.Header.Get("If-Match") != tag {
						t.Errorf("lost original version %q", tag)
					}
					resumedRanges.Add(1)
				}
				http.ServeContent(slowWriter{w, r.Context()}, r, "file.bin", time.Time{}, bytes.NewReader(data))
			}))
			defer server.Close()
			svc := downloadService(t)
			if _, err := svc.Add(t.Context(), server.URL+"/file.bin", svc.Snapshot().Directory); err != nil {
				t.Fatal(err)
			}
			id := svc.Snapshot().Tasks[0].ID
			awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Completed >= 512<<10 })
			if err := svc.Execute(t.Context(), id, application.Pause); err != nil {
				t.Fatal(err)
			}
			awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.CanResume })
			resumed.Store(true)
			if err := svc.Execute(t.Context(), id, application.Resume); err != nil {
				t.Fatal(err)
			}
			end := awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
			got, err := os.ReadFile(end.Path)
			if err != nil || !bytes.Equal(got, data) || resumedRanges.Load() == 0 || end.Status != "complete" {
				t.Fatalf("resume failed: %+v, %v", end, err)
			}
		})
	}
}

func TestReopenLegacyRefusalUsesOriginalCheckpoint(t *testing.T) {
	data := bytes.Repeat([]byte("abc"), 1<<20)
	var recovering, resumed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("ETag", `"download-revision"`)
		name := "initial.bin"
		if recovering.Load() {
			name = "new-name.bin"
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		if recovering.Load() && strings.HasPrefix(r.Header.Get("Range"), "bytes=") && !strings.HasPrefix(r.Header.Get("Range"), "bytes=0-") {
			resumed.Store(true)
		}
		http.ServeContent(slowWriter{w, r.Context()}, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	binary, err := aria2.FindBinary()
	if err != nil {
		t.Fatal(err)
	}
	proc, err := aria2.Start(t.Context(), t.TempDir(), binary)
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
	dir := t.TempDir()
	repo := persistence.Repository{Path: filepath.Join(t.TempDir(), "tasks.json")}
	newService := func() *application.Service {
		return application.New(proc.Client, repo, dir, func() string { return "0000000000000001" })
	}
	svc := newService()
	if err := svc.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(t.Context(), server.URL+"/download", dir); err != nil {
		t.Fatal(err)
	}
	id := svc.Snapshot().Tasks[0].ID
	task := awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Completed >= 128<<10 })
	if err := svc.Execute(t.Context(), id, application.Pause); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Status == "paused" })
	state, err := repo.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	row := &state.Tasks[0]
	row.Version = domain.SourceVersion{}
	row.Status = domain.Failed
	row.Failure = domain.SourceUnverified
	if err := repo.Save(t.Context(), state); err != nil {
		t.Fatal(err)
	}
	if err := proc.Client.Execute(t.Context(), id, application.Remove); err != nil {
		t.Fatal(err)
	}
	recovering.Store(true)
	svc = newService()
	if err := svc.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := svc.Snapshot().Tasks[0]; !got.CanResume {
		t.Fatal(got)
	}
	if err := svc.Execute(t.Context(), id, application.Resume); err != nil {
		t.Fatal(err)
	}
	end := awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
	got, err := os.ReadFile(task.Path)
	if end.Path != task.Path || end.Status != "complete" || !resumed.Load() || err != nil || !bytes.Equal(got, data) {
		t.Fatalf("checkpoint resume failed: %+v, requested missing data %t", end, resumed.Load())
	}
}

func TestCaptureValidatorFromGetWhenHEADUnavailable(t *testing.T) {
	const total = 20 << 20
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(405)
			return
		}
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Error("metadata probe requested full file")
		}
		w.Header().Set("Content-Range", "bytes 0-0/20971520")
		w.Header().Set("ETag", `"get-only"`)
		w.WriteHeader(206)
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()
	info, err := aria2.NewClient("", "").InspectSource(t.Context(), server.URL, true)
	if err != nil || info.ETag != `"get-only"` || info.RangeBytes != total {
		t.Fatal(info, err)
	}
}
