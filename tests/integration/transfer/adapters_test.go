//go:build integration

package transfer_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"strings"
	"testing"
)

func TestRepositoryRoundTripAndCorruptFileIsPreserved(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data", "tasks.json")
	repo := persistence.Repository{Path: path}
	if state, err := repo.Load(ctx); err != nil || len(state.Tasks) != 0 {
		t.Fatalf("fresh repo: %+v %v", state, err)
	}
	want := application.State{Version: 1, Preferences: application.Preferences{DefaultDirectory: t.TempDir(), StaticBackground: true, Theme: application.ThemeDark}, Directory: "/downloads", Tasks: []domain.Record{{ID: "id", URL: "https://example.com/a?secret=private", Status: domain.Paused, Completed: 7, Total: 100}}}
	if err := repo.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Load(ctx)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v %v", got, err)
	}
	assertPrivateTaskFile(t, path)
	_ = os.WriteFile(path, []byte("broken"), 0600)
	if _, err := repo.Load(ctx); err == nil {
		t.Fatal("corruption silently accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "broken" {
		t.Fatal("corrupt file overwritten")
	}
}
func TestRepositoryRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	_ = os.WriteFile(path, []byte(`{"Version":99,"Tasks":[]}`), 0600)
	if _, err := (persistence.Repository{Path: path}).Load(context.Background()); err == nil {
		t.Fatal("future schema accepted")
	}
}

func TestRPCEnqueueUsesSecretStableIDAndExplicitOptions(t *testing.T) {
	var request struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Error("wrong HTTP method")
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0000000000000001"}`))
	}))
	defer server.Close()
	dir := t.TempDir()
	err := aria2.NewClient(server.URL, "test-secret").Enqueue(context.Background(), application.TransferSpec{ID: "0000000000000001", URL: "https://example.com/a", Directory: dir, Paused: true})
	if err != nil {
		t.Fatal(err)
	}
	if request.Method != "aria2.addUri" || len(request.Params) != 3 || string(request.Params[0]) != `"token:test-secret"` {
		t.Fatalf("bad request %+v", request)
	}
	var options map[string]string
	_ = json.Unmarshal(request.Params[2], &options)
	if options["gid"] != "0000000000000001" || options["pause"] != "true" || options["dir"] != dir {
		t.Fatalf("bad options: %v", options)
	}
}

func TestDownloadDirectoryFailuresKeepTheirCodeAndFilesystemCause(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(dir, nil, 0600); err != nil {
		t.Fatal(err)
	}
	client := aria2.NewClient("", "")
	_, protectErr := client.ProtectCheckpoints(t.Context(), dir, nil)
	enqueueErr := client.Enqueue(t.Context(), application.TransferSpec{Directory: dir})
	for _, tc := range []struct {
		operation string
		err       error
	}{{"inspect", protectErr}, {"create", enqueueErr}} {
		t.Run(tc.operation, func(t *testing.T) {
			var cause *os.PathError
			if application.CodeOf(tc.err) != application.ErrDirectoryUnavailable || !errors.As(tc.err, &cause) || cause.Path != dir {
				t.Fatalf("directory failure lost its code or OS cause: %v", tc.err)
			}
		})
	}
}
func TestRPCPropagatesEngineErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":1,"message":"Unauthorized"}}`))
	}))
	defer server.Close()
	_, err := aria2.NewClient(server.URL, "wrong").Observe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("lost engine error: %v", err)
	}
}

func TestRemoveDoesNotTreatAuthenticationFailureAsMissingTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":1,"message":"Unauthorized"}}`))
	}))
	defer server.Close()
	err := aria2.NewClient(server.URL, "wrong").Execute(context.Background(), "0123456789abcdef", application.Remove)
	if err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("authentication failure was swallowed: %v", err)
	}
}
func TestRPCMapsProgressAndDoesNotLeakProtocolIntoApplication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Method string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := `[]`
		if req.Method == "aria2.tellActive" {
			result = `[{"gid":"stable","status":"active","totalLength":"100","completedLength":"25","downloadSpeed":"12","files":[{"path":"/downloads/server.zip"}]}]`
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))
	}))
	defer server.Close()
	got, err := aria2.NewClient(server.URL, "secret").Observe(context.Background())
	if err != nil || len(got) != 1 || got[0].ID != "stable" || got[0].Total != 100 || got[0].Completed != 25 || got[0].Name != "server.zip" {
		t.Fatalf("projection=%+v %v", got, err)
	}
}

func TestRPCClassifiesResumeFailureByCodeInsteadOfGenericMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Method string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := `[]`
		if req.Method == "aria2.tellStopped" {
			result = `[{"gid":"stable","status":"error","errorCode":"8","errorMessage":"No URI available."}]`
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))
	}))
	defer server.Close()
	got, err := aria2.NewClient(server.URL, "secret").Observe(context.Background())
	if err != nil || len(got) != 1 || got[0].Failure != "resume_unsupported" {
		t.Fatalf("lost structured failure: %+v, %v", got, err)
	}
}

func TestFreshDownloadUsesNewFilenameAndSingleConnection(t *testing.T) {
	var options map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Params []json.RawMessage }
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.Unmarshal(req.Params[2], &options)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0123456789abcdef"}`))
	}))
	defer server.Close()
	err := aria2.NewClient(server.URL, "secret").Enqueue(context.Background(), application.TransferSpec{
		ID: "0123456789abcdef", URL: "https://example.com/a.zip", Directory: t.TempDir(), OutputName: "a.restart-unique.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options["out"] != "a.restart-unique.zip" || options["continue"] != "false" || options["split"] != "1" || options["max-connection-per-server"] != "1" || options["allow-overwrite"] != "false" {
		t.Fatalf("unsafe restart options: %v", options)
	}
}

func TestRPCClassifiesInsufficientDiskSpaceByCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Method string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		result := `[]`
		if req.Method == "aria2.tellStopped" {
			result = `[{"gid":"disk-full","status":"error","errorCode":"9","errorMessage":"opaque engine message"}]`
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))
	}))
	defer server.Close()
	tasks, err := aria2.NewClient(server.URL, "secret").Observe(t.Context())
	if err != nil || len(tasks) != 1 || tasks[0].Failure != string(application.ErrInsufficientDiskSpace) {
		t.Fatalf("disk space failure was not classified: %+v, %v", tasks, err)
	}
}

func TestRPCPreservesDistinguishableDownloadFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Method string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		result := `[]`
		if req.Method == "aria2.tellStopped" {
			result = `[
				{"gid":"redirect","status":"error","errorCode":"23","errorMessage":"opaque"},
				{"gid":"auth","status":"error","errorCode":"24","errorMessage":"opaque"},
				{"gid":"checksum","status":"error","errorCode":"32","errorMessage":"opaque"},
				{"gid":"file","status":"error","errorCode":"16","errorMessage":"opaque"},
				{"gid":"directory","status":"error","errorCode":"18","errorMessage":"opaque"}
			]`
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))
	}))
	defer server.Close()
	tasks, err := aria2.NewClient(server.URL, "secret").Observe(t.Context())
	want := []string{
		string(application.ErrTooManyRedirects), string(application.ErrDownloadAuthorization), string(application.ErrDownloadChecksum),
		string(application.ErrCreateDownloadFile), string(application.ErrDirectoryUnavailable),
	}
	if err != nil || len(tasks) != len(want) {
		t.Fatalf("unexpected tasks: %+v, %v", tasks, err)
	}
	for i, code := range want {
		if tasks[i].Failure != code {
			t.Errorf("%s classified as %q, want %q", tasks[i].ID, tasks[i].Failure, code)
		}
	}
}
