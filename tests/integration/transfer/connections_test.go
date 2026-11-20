//go:build integration

package transfer_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"skiff/internal/contexts/transfer/application"
)

func TestConnectionPlanMatchesActualDownload(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		size, requested, expected int
		ranges                    bool
	}{
		{"small", 64 << 10, 0, 1, true}, {"ignored-range", 9 << 20, 0, 1, false}, {"manual", 12 << 20, 2, 2, true}, {"auto", 12 << 20, 0, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := bytes.Repeat([]byte{37}, tc.size)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.ranges {
					http.ServeContent(slowWriter{w, r.Context()}, r, "file.bin", time.Unix(1700000000, 0), bytes.NewReader(data))
					return
				}
				w.Header().Set("Content-Length", fmt.Sprint(len(data)))
				w.Header().Set("Accept-Ranges", "bytes")
				if r.Method != "HEAD" {
					_, _ = (slowWriter{w, r.Context()}).Write(data)
				}
			}))
			defer server.Close()
			svc := downloadService(t)
			result, err := svc.AddBatchWithConnections(t.Context(), server.URL+"/file.bin", svc.Snapshot().Directory, tc.requested)
			if err != nil || result.Added != 1 {
				t.Fatal(result, err)
			}
			task := svc.Snapshot().Tasks[0]
			if task.ConnectionLimit != tc.expected {
				t.Fatalf("limit %d, want %d", task.ConnectionLimit, tc.expected)
			}
			peak := 0
			done := awaitTask(t, svc, task.ID, func(v application.TaskDTO) bool { peak = max(peak, v.Connections); return v.Status == "complete" })
			if peak > tc.expected || tc.expected > 1 && peak < 2 {
				t.Fatalf("observed %d with limit %d", peak, tc.expected)
			}
			got, err := os.ReadFile(done.Path)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatal("incorrect output", err)
			}
		})
	}
}
