//go:build integration

package transfer_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"skiff/internal/contexts/transfer/application"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrivateHeadersStopAtOriginOnBothProbeAndDownload(t *testing.T) {
	var leaked, seen, gets, heads atomic.Int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie")+r.Header.Get("Authorization")+r.Header.Get("X-Custom")+r.Header.Get("Referer") != "" {
			leaked.Add(1)
		}
		if r.Method == "HEAD" {
			heads.Add(1)
		} else {
			gets.Add(1)
		}
		w.Header().Set("Content-Disposition", `attachment; filename="authorized.bin"`)
		w.Write([]byte("private file"))
	}))
	defer cdn.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=key" || r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("X-Custom") != "private" || r.UserAgent() != "private agent" {
			http.Error(w, "auth", 403)
			return
		}
		seen.Add(1)
		http.Redirect(w, r, cdn.URL+"/file", 302)
	}))
	defer src.Close()
	s := downloadService(t)
	_, err := s.AddWithOptions(t.Context(), src.URL+"/secret", s.Snapshot().Directory, application.AddOptions{Headers: "Cookie: session=key\nAuthorization: Bearer token\nX-Custom: private\nReferer: https://private.example/\nUser-Agent: private agent"})
	if err != nil {
		t.Fatal(err)
	}
	done := awaitTask(t, s, s.Snapshot().Tasks[0].ID, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
	data, _ := os.ReadFile(done.Path)
	if string(data) != "private file" || leaked.Load() != 0 || seen.Load() < 2 || gets.Load() == 0 || heads.Load() == 0 || done.Name != "authorized.bin" {
		t.Fatalf("task=%+v leaked=%d seen=%d", done, leaked.Load(), seen.Load())
	}
}

func TestForwardedDownloadKeepsRedirectFailureUntilRetry(t *testing.T) {
	var loop atomic.Bool
	loop.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=123-123" {
			w.Header().Set("Content-Range", "bytes 123-123/124")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("x"))
			return
		}
		if loop.Load() {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		http.Error(w, "source unavailable", http.StatusBadGateway)
	}))
	defer server.Close()
	svc, engine, state := downloadServiceWithClient(t)
	_, err := svc.AddWithOptions(t.Context(), server.URL+"/loop", svc.Snapshot().Directory, application.AddOptions{Headers: "X-Example: forwarded"})
	if err != nil {
		t.Fatal(err)
	}
	id := svc.Snapshot().Tasks[0].ID
	// Send a successful segment through the same local route after the failing
	// response, before the service observes and removes the route.
	uri := awaitFailedAria2URI(t, state, id)
	probe, err := http.NewRequestWithContext(t.Context(), http.MethodGet, uri, nil)
	if err != nil {
		t.Fatal(err)
	}
	probe.Header.Set("Range", "bytes=123-123")
	resp, err := http.DefaultClient.Do(probe)
	if err != nil {
		t.Fatal("could not send a later request to the local route")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("the later segment was not successful: %d", resp.StatusCode)
	}
	failed := func(task application.TaskDTO) bool { return task.Status == "error" }
	for range 2 {
		if task := awaitTask(t, svc, id, failed); task.Failure != string(application.ErrTooManyRedirects) {
			t.Errorf("forwarded task lost its redirect failure: %q", task.Failure)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := engine.Execute(ctx, id, application.Retry); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the retry RPC to fail: %v", err)
	}
	if task := awaitTask(t, svc, id, failed); task.Failure != string(application.ErrTooManyRedirects) {
		t.Errorf("rejected retry changed the previous failure: %q", task.Failure)
	}
	loop.Store(false)
	if err := svc.Execute(t.Context(), id, application.Retry); err != nil {
		t.Fatal(err)
	}
	if task := awaitTask(t, svc, id, failed); task.Failure != application.FailureSourceUnavailable {
		t.Errorf("retry reused the previous redirect failure: %q", task.Failure)
	}
}

func awaitFailedAria2URI(t *testing.T, state, id string) string {
	t.Helper()
	rpc := testAria2RPC(t, state)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(25 * time.Millisecond)
	defer poll.Stop()
	for {
		var task struct {
			Status string
			Files  []struct{ URIs []struct{ URI string } }
		}
		rpc("tellStatus", &task, id, []string{"status", "files"})
		if task.Status == "error" {
			if len(task.Files) == 0 || len(task.Files[0].URIs) == 0 {
				t.Fatal("aria2 stopped reporting the local route")
			}
			return task.Files[0].URIs[0].URI
		}
		select {
		case <-deadline.C:
			t.Fatal("aria2 did not report a failed download")
		case <-poll.C:
		}
	}
}
