//go:build integration

package transfer_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"testing"
)

func TestSingleOutputNameAndRecentDestination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("new contents")) }))
	defer srv.Close()
	s := downloadService(t)
	base, dir := t.TempDir(), t.TempDir()
	if err := s.SetDefaultDirectory(t.Context(), base); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "我的文件.zip")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := application.AddOptions{OutputName: "我的文件.zip"}
	if _, err := s.AddWithOptions(t.Context(), srv.URL+"/one", dir, opts); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if snap.Directory != base || snap.Preferences.RecentDirectories[0] != dir {
		t.Fatal("lost default or recently used directory")
	}
	done := awaitTask(t, s, snap.Tasks[0].ID, func(task application.TaskDTO) bool { return task.Status == "complete" })
	actual, _ := os.ReadFile(done.Path)
	kept, _ := os.ReadFile(path)
	if done.Path == path || string(actual) != "new contents" || string(kept) != "original" {
		t.Fatal(done.Path, string(actual), string(kept))
	}
	if _, err := s.AddWithOptions(t.Context(), srv.URL+"/one\n"+srv.URL+"/two", dir, opts); err == nil {
		t.Fatal("batch accepted a shared filename")
	}
	opts.OutputName = "../escape"
	if _, err := s.AddWithOptions(t.Context(), srv.URL+"/two", dir, opts); err == nil {
		t.Fatal("path in filename accepted")
	}
}
