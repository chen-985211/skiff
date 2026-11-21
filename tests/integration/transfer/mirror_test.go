//go:build integration

package transfer_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"skiff/internal/contexts/transfer/application"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMirrorFallsBackWithoutMixingPartialFiles(t *testing.T) {
	for _, verified := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown", true: "verified"}[verified], func(t *testing.T) {
			var unavailable, resumed atomic.Bool
			data := bytes.Repeat([]byte("first"), 1<<20)
			other := bytes.Repeat([]byte("other"), 1<<20)
			var digest string
			if verified {
				other = data
				hash := sha256.Sum256(data)
				digest = "sha-256=:" + base64.StdEncoding.EncodeToString(hash[:]) + ":"
			}
			main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if unavailable.Load() {
					http.NotFound(w, r)
					return
				}
				if digest != "" {
					w.Header().Set("Repr-Digest", digest)
				}
				w.Header().Set("ETag", `"same"`)
				http.ServeContent(slowWriter{w, r.Context()}, r, "file", time.Time{}, bytes.NewReader(data))
			}))
			defer main.Close()
			mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if digest != "" {
					w.Header().Set("Repr-Digest", digest)
				}
				w.Header().Set("ETag", `"same"`)
				if r.Method == "GET" && strings.HasPrefix(r.Header.Get("Range"), "bytes=") && !strings.HasPrefix(r.Header.Get("Range"), "bytes=0-") {
					resumed.Store(true)
				}
				http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(other))
			}))
			defer mirror.Close()
			s := downloadService(t)
			_, err := s.AddWithOptions(t.Context(), main.URL+"/file", s.Snapshot().Directory, application.AddOptions{Mirrors: mirror.URL + "/backup", ConnectionLimit: 1})
			if err != nil {
				t.Fatal(err)
			}
			id := s.Snapshot().Tasks[0].ID
			first := awaitTask(t, s, id, func(v application.TaskDTO) bool { return v.Completed >= 1<<20 && v.Status == "active" })
			if err := s.Execute(t.Context(), id, application.Pause); err != nil {
				t.Fatal(err)
			}
			awaitTask(t, s, id, func(v application.TaskDTO) bool { return v.Status == "paused" })
			unavailable.Store(true)
			if err := s.Execute(t.Context(), id, application.Resume); err != nil {
				t.Fatal(err)
			}
			done := awaitTask(t, s, id, func(v application.TaskDTO) bool { return v.Status == "complete" })
			result, _ := os.ReadFile(done.Path)
			before, _ := os.ReadFile(first.Path)
			if (first.Path == done.Path) != verified || verified && !resumed.Load() || !bytes.Equal(result, other) || len(before) == 0 || done.URL != main.URL+"/file" {
				t.Fatal("unverified mirror reused or removed primary file", done)
			}
		})
	}
}
