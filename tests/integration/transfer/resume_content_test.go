//go:build integration

package transfer_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/domain"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"sync/atomic"
	"testing"
	"time"

	"skiff/internal/contexts/transfer/application"
)

// HEAD provides no version validator. The two representations even have the
// same length and agree at both ends of the partial file.
func TestResumeChecksSavedBytesWhenSourceHasNoValidator(t *testing.T) {
	for _, change := range []bool{false, true} {
		name := "unchanged"
		if change {
			name = "changed-interior"
		}
		t.Run(name, func(t *testing.T) {
			original := bytes.Repeat([]byte("download"), 512<<10)
			replaced := bytes.Clone(original)
			replaced[200<<10] ^= 1
			var switched atomic.Bool
			var resumedRanges atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				content := original
				if switched.Load() {
					if change {
						content = replaced
					}
					if r.Header.Get("Range") != "" {
						resumedRanges.Add(1)
					}
				}
				http.ServeContent(slowWriter{w, r.Context()}, r, "file.bin", time.Time{}, bytes.NewReader(content))
			}))
			defer server.Close()
			svc := downloadService(t)
			if _, err := svc.Add(t.Context(), server.URL+"/file.bin", svc.Snapshot().Directory); err != nil {
				t.Fatal(err)
			}
			id := svc.Snapshot().Tasks[0].ID
			task := awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Completed >= 512<<10 })
			if err := svc.Execute(t.Context(), id, application.Pause); err != nil {
				t.Fatal(err)
			}
			awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.CanResume })
			before, err := os.ReadFile(task.Path)
			if err != nil {
				t.Fatal(err)
			}
			switched.Store(true)
			if err := svc.Execute(t.Context(), id, application.Resume); err != nil {
				t.Fatal(err)
			}
			end := awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
			got, err := os.ReadFile(task.Path)
			if err != nil {
				t.Fatal(err)
			}
			if resumedRanges.Load() == 0 {
				t.Fatal("resume rejected before checking any bytes")
			}
			if change {
				if !end.CanRestart || !bytes.Equal(got, before) {
					t.Fatal("different partial was reused", end)
				}
			} else if end.Status != "complete" || end.Path != task.Path || !bytes.Equal(got, original) || resumedRanges.Load() < 2 {
				t.Fatal("did not resume original file", end)
			}
		})
	}
}

func TestReopenLegacyRefusalPinsVersionReturnedByRange(t *testing.T) {
	data := bytes.Repeat([]byte("abc"), 1<<20)
	var recovering, pinned atomic.Bool
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
		if recovering.Load() && r.Header.Get("If-Match") == `"download-revision"` && r.Header.Get("Range") != "" {
			pinned.Store(true)
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
	if end.Path != task.Path || end.Status != "complete" || !pinned.Load() || err != nil || !bytes.Equal(got, data) {
		t.Fatalf("checkpoint resume/pinning failed: %+v, pinned %t", end, pinned.Load())
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

func TestSparseCheckpointChecksEveryReusableBlockWithoutReadingHoles(t *testing.T) {
	const piece = 32 << 10
	data := bytes.Repeat([]byte{17}, 3*piece)
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Header.Get("Range")
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	partial := bytes.Clone(data)
	clear(partial[16<<10 : 2*piece])
	if err := os.WriteFile(path, partial, 0600); err != nil {
		t.Fatal(err)
	}
	var checkpoint bytes.Buffer
	fields := []any{uint16(1), uint32(0), uint32(0), uint32(piece), uint64(len(data)), uint64(0), uint32(1), byte(0x20), uint32(1), uint32(0), uint32(piece), uint32(1), byte(0x80)}
	for _, v := range fields {
		if err := binary.Write(&checkpoint, binary.BigEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	ctrl := path + ".aria2"
	if err := os.WriteFile(ctrl, checkpoint.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	client := aria2.NewClient("", "")
	r := domain.Record{URL: server.URL, Directory: dir, Path: path, Status: domain.Failed, Total: int64(len(data))}
	result, err := client.CheckPartial(t.Context(), r)
	if err != nil || result.Failure != "" {
		t.Fatal(result, err)
	}
	for _, want := range []string{"bytes=0-16383", "bytes=65536-98303"} {
		select {
		case got := <-requests:
			if got != want {
				t.Fatalf("range %s, want %s", got, want)
			}
		default:
			t.Fatal("missing range", want)
		}
	}
	for _, bad := range [][]byte{checkpoint.Bytes()[:16], append(bytes.Clone(checkpoint.Bytes()), 1)} {
		if err := os.WriteFile(ctrl, bad, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := client.CheckPartial(t.Context(), r)
		if err != nil || got.Failure != domain.CheckpointUnverified {
			t.Fatalf("bad checkpoint accepted: %+v, %v", got, err)
		}
		select {
		case <-requests:
			t.Fatal("invalid checkpoint caused network request")
		default:
		}
	}
}
