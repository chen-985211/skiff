//go:build integration

package transfer_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProxyAndDNSForMetadataAndDownload(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "download.invalid" || r.Header.Get("Proxy-Authorization") != "Basic ZGVtbzprZXk=" {
			http.Error(w, "proxy auth", 407)
			return
		}
		hits.Add(1)
		w.Write([]byte("proxied file"))
	}))
	defer proxy.Close()
	cfg := application.NetworkSettings{Mode: "manual", Proxy: proxy.URL, User: "demo", Password: "key", Bypass: "127.0.0.1,.internal", DNS: "system"}
	s, rpc := configuredService(t, application.Preferences{})
	if err := s.SetNetwork(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(t.Context(), "http://download.invalid/file", s.Snapshot().Directory); err != nil {
		t.Fatal(err)
	}
	task := s.Snapshot().Tasks[0]
	done := awaitTask(t, s, task.ID, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
	data, _ := os.ReadFile(done.Path)
	if string(data) != "proxied file" || hits.Load() < 2 {
		t.Fatal("proxy did not cover both requests", done, hits.Load())
	}
	if rpc("getOption", task.ID)["async-dns"] != "false" {
		t.Fatal("system DNS not applied")
	}
}
func TestProxyBypassAndPerTaskOverride(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "bypass", true: "override"}[override], func(t *testing.T) {
			s, _ := configuredService(t, application.Preferences{})
			cfg := application.NetworkSettings{Mode: "manual", Proxy: "http://127.0.0.1:1", Bypass: "127.0.0.1"}
			opts := application.AddOptions{}
			if override {
				cfg.Bypass = ""
				opts.Network = application.NetworkSettings{Mode: "direct"}
			}
			if e := s.SetNetwork(t.Context(), cfg); e != nil {
				t.Fatal(e)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("direct")) }))
			defer srv.Close()
			if _, e := s.AddWithOptions(t.Context(), srv.URL+"/file", s.Snapshot().Directory, opts); e != nil {
				t.Fatal(e)
			}
			done := awaitTask(t, s, s.Snapshot().Tasks[0].ID, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
			data, _ := os.ReadFile(done.Path)
			if strings.TrimSpace(string(data)) != "direct" {
				t.Fatal(done)
			}
		})
	}
}

func TestPausedProxiedDownloadRebindsAfterRelaunch(t *testing.T) {
	data := bytes.Repeat([]byte("download"), 512<<10)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(slowWriter{w, r.Context()}, r, "file.bin", time.Unix(1700000000, 0), bytes.NewReader(data))
	}))
	defer proxy.Close()
	state, target := t.TempDir(), t.TempDir()
	repo := persistence.Repository{Path: filepath.Join(state, "tasks.json")}
	start := func() (*application.Service, func()) {
		t.Helper()
		binary, e := aria2.FindBinary()
		if e != nil {
			t.Fatal(e)
		}
		p, e := aria2.Start(t.Context(), state, binary)
		if e != nil {
			t.Fatal(e)
		}
		closeProcess := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if e := p.Close(ctx); e != nil {
				t.Error(e)
			}
		}
		t.Cleanup(closeProcess)
		s := application.New(p.Client, repo, target, func() string { return "0123456789abcd11" })
		if e := s.Open(t.Context()); e != nil {
			t.Fatal(e)
		}
		return s, closeProcess
	}
	s, closeProcess := start()
	if _, e := s.AddWithOptions(t.Context(), "http://fixture.invalid/file.bin", target, application.AddOptions{Network: application.NetworkSettings{Mode: "manual", Proxy: proxy.URL}}); e != nil {
		t.Fatal(e)
	}
	id := s.Snapshot().Tasks[0].ID
	awaitTask(t, s, id, func(v application.TaskDTO) bool { return v.Completed > 0 && v.Status == "active" })
	if e := s.Execute(t.Context(), id, application.Pause); e != nil {
		t.Fatal(e)
	}
	closeProcess()
	s, _ = start()
	if e := s.Execute(t.Context(), id, application.Resume); e != nil {
		t.Fatal(e)
	}
	done := awaitTask(t, s, id, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
	actual, _ := os.ReadFile(done.Path)
	if !bytes.Equal(data, actual) {
		t.Fatal("restored download failed", done)
	}
}
