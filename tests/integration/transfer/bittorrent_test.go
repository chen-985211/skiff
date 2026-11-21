//go:build integration

package transfer_test

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
)

func btService(t *testing.T) (*application.Service, *aria2.Process, string) {
	t.Helper()
	bin, err := aria2.FindBinary()
	if err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	dir := t.TempDir()
	p, err := aria2.Start(t.Context(), data, bin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := p.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	var seq int
	s := application.New(p.Client, persistence.Repository{Path: filepath.Join(data, "tasks.json")}, dir, func() string { seq++; return fmt.Sprintf("%016x", seq) })
	if err := s.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	return s, p, dir
}
func btFixture(t *testing.T) (string, string) {
	t.Helper()
	a, b := strings.Repeat("a", 16384), strings.Repeat("b", 16384)
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]string{"/set/a.txt": a, "/set/sub/b.txt": b}[r.URL.Path]
		if data == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(data))
	}))
	t.Cleanup(web.Close)
	x, y := sha1.Sum([]byte(a)), sha1.Sum([]byte(b))
	info := "d5:filesld6:lengthi16384e4:pathl5:a.txteed6:lengthi16384e4:pathl3:sub5:b.txteee4:name3:set12:piece lengthi16384e6:pieces40:" + string(x[:]) + string(y[:]) + "e"
	data := "d4:info" + info + fmt.Sprintf("8:url-list%d:%se", len(web.URL)+1, web.URL+"/")
	file := filepath.Join(t.TempDir(), "fixture.torrent")
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return file, fmt.Sprintf("magnet:?xt=urn:btih:%x", sha1.Sum([]byte(info)))
}
func TestBTSelectedFileDownloadsAndMagnetUsesMatchingMetadata(t *testing.T) {
	s, _, dir := btService(t)
	file, magnet := btFixture(t)
	meta, err := s.PrepareTorrent(t.Context(), file)
	if err != nil || len(meta.Files) != 2 || meta.Files[1].Path != "sub/b.txt" {
		t.Fatalf("inspection: %+v %v", meta, err)
	}
	cached, err := s.PrepareTorrent(t.Context(), magnet)
	if err != nil || meta.Hash != cached.Hash {
		t.Fatalf("magnet: %v", err)
	}
	if _, err = s.AddTorrent(t.Context(), meta.Hash, dir, nil); err == nil {
		t.Fatal("empty selection accepted")
	}
	id, err := s.AddTorrent(t.Context(), meta.Hash, dir, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	task := awaitTask(t, s, id, func(t application.TaskDTO) bool { return t.Status == "complete" || t.Status == "error" })
	b, err := os.ReadFile(filepath.Join(dir, "set", "sub", "b.txt"))
	if err != nil || len(b) != 16384 || b[0] != 'b' || task.Name != "set" || task.Total != 16384 {
		t.Fatalf("selected download: %+v, %v", task, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "set", "a.txt")); !os.IsNotExist(err) {
		t.Fatal("unselected file was downloaded")
	}
}

func TestBTMagnetFetchesOnlyMetadataFromLoopbackPeer(t *testing.T) {
	s, _, dir := btService(t)
	file, magnet := btFixture(t)
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	peers := []byte{127, 0, 0, 1, byte(port >> 8), byte(port)}
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "d8:intervali1e5:peers6:%se", peers) }))
	t.Cleanup(tracker.Close)
	data := append([]byte(fmt.Sprintf("d8:announce%d:%s", len(tracker.URL), tracker.URL)), raw[1:]...)
	if err = os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	origin := t.TempDir()
	if err = os.MkdirAll(filepath.Join(origin, "set/sub"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct{ name, char string }{{"a.txt", "a"}, {"sub/b.txt", "b"}} {
		if err = os.WriteFile(filepath.Join(origin, "set", v.name), []byte(strings.Repeat(v.char, 16384)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	binary, err := aria2.FindBinary()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--no-conf=true", "--enable-dht=false", "--enable-dht6=false", "--bt-enable-lpd=false", "--enable-peer-exchange=false", "--bt-hash-check-seed=true", "--check-integrity=true", "--seed-ratio=0", "--seed-time=10", "--listen-port="+strconv.Itoa(port), "--dir="+origin, file)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); _ = cmd.Wait() }()
	meta, err := s.PrepareTorrent(ctx, magnet+"&tr="+url.QueryEscape(tracker.URL))
	if err != nil || len(meta.Files) != 2 {
		t.Fatalf("peer metadata: %v %+v", err, meta)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("magnet fetched payload before selection: %v %v", entries, err)
	}
}
