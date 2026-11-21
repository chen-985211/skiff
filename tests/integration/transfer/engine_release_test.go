//go:build integration && engine_release

package transfer_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
)

// CI supplies the untouched upstream archive for the native runner. Only the
// download host changes; platform selection and both pinned digests stay live.
func TestNativeEngineReleaseDownloadsFile(t *testing.T) {
	archive := os.Getenv("SKIFF_TEST_ENGINE_ARCHIVE")
	if _, err := os.Stat(archive); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKIFF_ARIA2_BINARY", "")
	dir := t.TempDir()
	setup := aria2.NewInstallation(dir)
	if setup.Release.URL == "" || path.Base(setup.Release.URL) != filepath.Base(archive) {
		t.Fatalf("release %q does not match fixture %q", setup.Release.URL, archive)
	}
	source := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir(archive))))
	defer source.Close()
	setup.Release.URL = source.URL + "/" + path.Base(setup.Release.URL)
	var installed string
	var err error
	if runtime.GOOS == "darwin" {
		// The local Mac may already have Homebrew's engine.
		installed, err = setup.Install(t.Context())
	} else {
		t.Setenv("PATH", t.TempDir())
		if _, err := setup.Find(); !errors.Is(err, application.ErrEngineMissing) {
			t.Fatalf("expected the download button on a fresh install: %v", err)
		}
		clicks := make(chan struct{}, 1)
		installed, err = application.PrepareEngine(t.Context(), setup, clicks, func(reason error) {
			if reason != nil {
				t.Fatal(reason)
			}
			clicks <- struct{}{}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	source.Close()
	binary, err := aria2.NewInstallation(dir).Find()
	if err != nil || binary != installed {
		t.Fatalf("restart found %q instead of %q: %v", binary, installed, err)
	}

	t.Setenv("SKIFF_ARIA2_BINARY", binary)
	service := downloadService(t)
	content := []byte("Skiff native engine download fixture\n")
	file := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "fixture.bin", time.Time{}, bytes.NewReader(content))
	}))
	defer file.Close()
	if _, err := service.Add(t.Context(), file.URL+"/fixture.bin", service.Snapshot().Directory); err != nil {
		t.Fatal(err)
	}
	task := service.Snapshot().Tasks[0]
	done := awaitTask(t, service, task.ID, func(task application.TaskDTO) bool { return task.Status == "complete" })
	got, err := os.ReadFile(done.Path)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("downloaded %q: %v", got, err)
	}
}
