//go:build integration

package transfer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"strings"
	"testing"
	"time"
)

func configuredService(t *testing.T, prefs application.Preferences) (*application.Service, func(string, ...any) map[string]string) {
	t.Helper()
	dir := t.TempDir()
	binary, e := aria2.FindBinary()
	if e != nil {
		t.Fatal(e)
	}
	proc, e := aria2.StartConfigured(t.Context(), dir, binary, prefs)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if e := proc.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	repo := persistence.Repository{Path: filepath.Join(dir, "tasks.json")}
	if err := repo.Save(t.Context(), application.State{Version: 1, Preferences: prefs}); err != nil {
		t.Fatal(err)
	}
	s := application.New(proc.Client, repo, t.TempDir(), func() string { return "0123456789abcdef" })
	if err := s.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	conf, err := os.ReadFile(filepath.Join(dir, "aria2.conf"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{}
	for line := range strings.Lines(string(conf)) {
		k, v, _ := strings.Cut(strings.TrimSpace(line), "=")
		cfg[k] = v
	}
	return s, func(method string, args ...any) map[string]string {
		t.Helper()
		params := append([]any{"token:" + cfg["rpc-secret"]}, args...)
		body, _ := json.Marshal(map[string]any{"id": "test", "jsonrpc": "2.0", "method": "aria2." + method, "params": params})
		resp, e := http.Post("http://127.0.0.1:"+cfg["rpc-listen-port"]+"/jsonrpc", "application/json", bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		var out struct{ Result map[string]string }
		if e := json.NewDecoder(resp.Body).Decode(&out); e != nil || out.Result == nil {
			t.Fatal(method, e)
		}
		return out.Result
	}
}
func TestDiskSettingsReachEngine(t *testing.T) {
	disk := application.DiskSettings{CacheMiB: 32, SplitMiB: 1, Allocation: "prealloc"}
	s, rpc := configuredService(t, application.Preferences{Disk: disk})
	if rpc("getGlobalOption")["disk-cache"] != "33554432" {
		t.Fatal("wrong startup cache")
	}
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(slowWriter{w, r.Context()}, r, "file", time.Time{}, bytes.NewReader(make([]byte, 6<<20)))
	}))
	defer src.Close()
	if _, err := s.Add(t.Context(), src.URL+"/file", s.Snapshot().Directory); err != nil {
		t.Fatal(err)
	}
	task := s.Snapshot().Tasks[0]
	opts := rpc("getOption", task.ID)
	if opts["min-split-size"] != "1048576" || opts["file-allocation"] != "prealloc" || task.ConnectionLimit != 4 {
		t.Fatal("incorrect segmentation/allocation", opts["min-split-size"], opts["file-allocation"], task.ConnectionLimit)
	}
	if err := s.SetDiskSettings(t.Context(), application.DiskSettings{}); err != nil || s.Snapshot().Preferences.Disk != (application.DiskSettings{}) {
		t.Fatal("failed to restore defaults", err)
	}
}
