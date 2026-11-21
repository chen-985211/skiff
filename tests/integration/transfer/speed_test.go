//go:build integration

package transfer_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"skiff/internal/contexts/transfer/application"
	"testing"
	"time"
)

func TestSpeedLimitsChangeOnRunningEngine(t *testing.T) {
	s, rpc := configuredService(t, application.Preferences{DownloadLimit: 2 << 20})
	if rpc("getGlobalOption")["max-overall-download-limit"] != "2097152" {
		t.Fatal("saved global cap missed on startup")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(slowWriter{w, r.Context()}, r, "file", time.Time{}, bytes.NewReader(make([]byte, 3<<20)))
	}))
	defer srv.Close()
	if _, e := s.Add(t.Context(), srv.URL+"/file", s.Snapshot().Directory); e != nil {
		t.Fatal(e)
	}
	id := s.Snapshot().Tasks[0].ID
	if err := s.SetDownloadLimit(t.Context(), id, 512<<10); err != nil {
		t.Fatal(err)
	}
	if rpc("getOption", id)["max-download-limit"] != "524288" || s.Snapshot().Tasks[0].DownloadLimit != 512<<10 {
		t.Fatal("task cap missing")
	}
	if err := s.SetDownloadLimit(t.Context(), "", 0); err != nil {
		t.Fatal(err)
	}
	if rpc("getGlobalOption")["max-overall-download-limit"] != "0" || rpc("getOption", id)["max-download-limit"] != "524288" {
		t.Fatal("limits not independent")
	}
	if err := s.SetDownloadLimit(t.Context(), id, 0); err != nil || rpc("getOption", id)["max-download-limit"] != "0" {
		t.Fatal("failed to remove task cap", err)
	}
}
