//go:build integration

package transfer_test

import (
	"net/http"
	"net/http/httptest"
	"skiff/internal/contexts/transfer/application"
	"strings"
	"testing"
	"time"
)

func TestQueueLimitAndPriorityChooseNextActualTransfer(t *testing.T) {
	start := make(chan string, 5)
	gates := map[string]chan struct{}{"/first": make(chan struct{}), "/second": make(chan struct{}), "/third": make(chan struct{})}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "2")
		if r.Method == "HEAD" {
			return
		}
		start <- r.URL.Path
		select {
		case <-gates[r.URL.Path]:
			_, _ = w.Write([]byte("ok"))
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	s := downloadService(t)
	if err := s.SetConcurrency(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	sources := []string{server.URL + "/first", server.URL + "/second", server.URL + "/third"}
	if _, err := s.Add(t.Context(), strings.Join(sources, "\n"), s.Snapshot().Directory); err != nil {
		t.Fatal(err)
	}
	expect := func(want string) {
		t.Helper()
		select {
		case path := <-start:
			if path != want {
				t.Fatalf("started %s, want %s", path, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("never started %s", want)
		}
	}
	expect("/first")
	tasks := s.Snapshot().Tasks
	awaitTask(t, s, tasks[2].ID, func(task application.TaskDTO) bool { return task.CanPrioritize })
	if err := s.Execute(t.Context(), tasks[2].ID, application.Prioritize); err != nil {
		t.Fatal(err)
	}
	close(gates["/first"])
	expect("/third")
	if err := s.SetConcurrency(t.Context(), 2); err != nil {
		t.Fatal(err)
	}
	expect("/second")
	close(gates["/second"])
	close(gates["/third"])
	for _, task := range tasks {
		awaitTask(t, s, task.ID, func(task application.TaskDTO) bool { return task.Status == "complete" })
	}
}
