//go:build integration

package transfer_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skiff/internal/contexts/transfer/infrastructure/aria2"
)

func engineArchive(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	if filepath.Ext(name) == ".exe" {
		z := zip.NewWriter(&b)
		f, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(data); err != nil {
			t.Fatal(err)
		}
		if err = z.Close(); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	gz := gzip.NewWriter(&b)
	tr := tar.NewWriter(gz)
	if err := tr.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestEngineUsesSystemOrVerifiedInstallation(t *testing.T) {
	t.Setenv("SKIFF_ARIA2_BINARY", "")
	if system, err := aria2.FindBinary(); err == nil {
		if got, err := aria2.NewInstallation(t.TempDir()).Find(); err != nil || got != system {
			t.Fatalf("expected system engine %q, got %q: %v", system, got, err)
		}
	}
	for _, artifact := range []struct{ binary, archive string }{
		{"aria2c", "engine.tar.gz"},
		{"aria2c.exe", "engine.zip"},
	} {
		t.Run(artifact.archive, func(t *testing.T) {
			dir := t.TempDir()
			setup := aria2.NewInstallation(dir)
			payload := []byte("engine fixture")
			archive := engineArchive(t, artifact.binary, payload)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(archive) }))
			defer server.Close()
			setup.Client = server.Client()
			setup.Release = aria2.EngineRelease{URL: server.URL + "/" + artifact.archive, ArchiveSHA256: fmt.Sprintf("%x", sha256.Sum256(archive)), BinarySHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}
			path, err := setup.Install(t.Context())
			if err != nil || path != filepath.Join(dir, "engine", artifact.binary) {
				t.Fatalf("installed %q: %v", path, err)
			}
			server.Close()
			if got, err := setup.Find(); err != nil || got != path {
				t.Fatalf("restart found %q: %v", got, err)
			}
			info, err := os.Stat(path)
			if err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0100 == 0 {
				t.Fatalf("not executable: %v, %v", info, err)
			}
			b, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(b, payload) {
				t.Fatal("wrong installed binary", err)
			}
			if err := os.WriteFile(path, []byte("corrupt cached binary"), 0700); err != nil {
				t.Fatal(err)
			}
			if got, _ := setup.Find(); got == path {
				t.Fatal("reused a damaged engine")
			}
			t.Setenv("SKIFF_ARIA2_BINARY", filepath.Join(dir, "explicit-missing"))
			if _, err := setup.Find(); err == nil {
				t.Fatal("ignored explicit engine override")
			}
		})
	}
}

func TestEngineRejectsIncompleteOrUnexpectedArtifacts(t *testing.T) {
	payload := []byte("expected")
	valid := engineArchive(t, "aria2c", payload)
	for _, tc := range []struct {
		name   string
		body   []byte
		digest []byte
		status int
	}{
		{"bad digest", valid, []byte("expected archive"), 200},
		{"short response", valid[:len(valid)/2], valid, 200},
		{"wrong executable", engineArchive(t, "aria2c", []byte("other")), nil, 200},
		{"wrong member", engineArchive(t, "../aria2c", payload), nil, 200},
		{"wrong zip member", engineArchive(t, "../aria2c.exe", payload), nil, 200},
		{"damaged exe", engineArchive(t, "aria2c.exe", []byte("other")), nil, 200},
		{"http error", valid, valid, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); _, _ = w.Write(tc.body) }))
			defer server.Close()
			if tc.digest == nil {
				tc.digest = tc.body
			}
			dir := t.TempDir()
			setup := aria2.NewInstallation(dir)
			setup.Client = server.Client()
			suffix := ".tar.gz"
			if bytes.HasPrefix(tc.body, []byte("PK")) {
				suffix = ".zip"
			}
			setup.Release = aria2.EngineRelease{URL: server.URL + "/engine" + suffix, ArchiveSHA256: fmt.Sprintf("%x", sha256.Sum256(tc.digest)), BinarySHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}
			if _, err := setup.Install(t.Context()); err == nil {
				t.Fatal("accepted invalid release")
			}
			entries, err := os.ReadDir(filepath.Join(dir, "engine"))
			if len(entries) != 0 || (err != nil && !os.IsNotExist(err)) {
				t.Fatalf("partial install: %v %v", entries, err)
			}
		})
	}
}

func TestEngineInterruptedDownloadPreservesInstalledFile(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	dir := t.TempDir()
	name := "aria2c"
	if runtime.GOOS == "windows" {
		name = "aria2c.exe"
	}
	path := filepath.Join(dir, "engine", name)
	if err := os.Mkdir(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("previous"), 0700); err != nil {
		t.Fatal(err)
	}
	setup := aria2.NewInstallation(dir)
	setup.Release.URL = server.URL
	setup.Client = server.Client()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := setup.Install(ctx); done <- err }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("request never started: %v", err)
	case <-t.Context().Done():
		t.Fatal("timed out waiting for request")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled install: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "previous" {
		t.Fatal("interrupted download changed the engine", err)
	}
}

// Some transports reach EOF after cancellation instead of returning a read error.
type canceledEngineResponse struct{ cancel context.CancelFunc }

func (r canceledEngineResponse) RoundTrip(*http.Request) (*http.Response, error) {
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		_, _ = io.WriteString(writer, "partial archive")
		r.cancel()
	}()
	return &http.Response{StatusCode: http.StatusOK, Body: reader}, nil
}

func TestCanceledEngineResponseEndingAtEOFReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	setup := aria2.NewInstallation(t.TempDir())
	setup.Release.URL = "https://engine.test/aria2c.tar.gz"
	setup.Client = &http.Client{Transport: canceledEngineResponse{cancel}}
	if _, err := setup.Install(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("partial response after cancellation: %v", err)
	}
}
