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
	"sync/atomic"
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
	p, err := aria2.StartConfigured(t.Context(), data, bin, application.Preferences{BT: application.BTSettings{DisableDHT: true}})
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
	if err := s.SetTorrentPeerLimit(t.Context(), id, 17); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Tasks[0].PeerLimit != 17 {
		t.Fatal("individual peer limit was not saved")
	}
	task := awaitTask(t, s, id, func(t application.TaskDTO) bool { return t.Status == "complete" || t.Status == "error" })
	b, err := os.ReadFile(filepath.Join(dir, "set", "sub", "b.txt"))
	if err != nil || len(b) != 16384 || b[0] != 'b' || task.Name != "set" || task.Total != 16384 {
		t.Fatalf("selected download: %+v, %v", task, err)
	}
	details, err := s.TorrentDetails(t.Context(), id)
	if err != nil || details.Pieces != 2 || details.Bitfield == "" {
		t.Fatalf("missing BT piece telemetry: %+v, %v", details, err)
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

func TestBTSubscriptionsRefreshDeduplicateAndRetainLastWorkingList(t *testing.T) {
	s, _, _ := btService(t)
	var broken atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if broken.Load() {
			http.Error(w, "offline", 503)
			return
		}
		fmt.Fprintln(w, "udp://127.0.0.1:9090/announce\n\nhttp://127.0.0.1:7070/announce\nudp://127.0.0.1:9090/announce")
	}))
	defer server.Close()
	cfg := application.BTSettings{Sources: server.URL, MaxPeers: 12, ListenPort: 9870}
	if err := s.SetBTSettings(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshBTSubscriptions(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	sub := s.Snapshot().BTSubscriptions
	if len(sub) != 1 || len(sub[0].Trackers) != 2 || sub[0].Error != "" {
		t.Fatal(sub)
	}
	broken.Store(true)
	if err := s.RefreshBTSubscriptions(t.Context(), true); err == nil {
		t.Fatal("failed source reported healthy")
	}
	sub = s.Snapshot().BTSubscriptions
	if len(sub[0].Trackers) != 2 || sub[0].Error == "" {
		t.Fatal("lost cached trackers or health", sub)
	}
}

func TestBTRelocateVerifiedFilesMoveAndResumeSeeding(t *testing.T) {
	s, _, dir := btService(t)
	ctx := t.Context()
	source, _ := btFixture(t)
	meta, err := s.PrepareTorrent(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.AddTorrent(ctx, meta.Hash, dir, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	awaitTask(t, s, id, func(t application.TaskDTO) bool { return t.Status == "complete" })
	wrong := t.TempDir()
	if err := s.UseTorrentFiles(ctx, id, wrong, false, false); err == nil || s.Snapshot().Tasks[0].Directory != dir {
		t.Fatal("adopted invalid files")
	}
	unrelated := filepath.Join(dir, "set", "notes")
	if err := os.WriteFile(unrelated, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err = s.UseTorrentFiles(ctx, id, dest, true, false); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Tasks[0]; got.Directory != dest || got.ID != id || got.Status != "complete" {
		t.Fatal(got)
	}
	if _, err = os.Stat(filepath.Join(dir, "set", "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("old payload: %v", err)
	}
	if _, err = os.Stat(unrelated); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dest, "set", "a.txt")
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, []byte(strings.Repeat("x", len(original))), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.VerifyTorrentFiles(ctx, id); err == nil {
		t.Fatal("accepted wrong piece with identical size")
	}
	if err = os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := s.Snapshot().Preferences.BT
	cfg.Seed = application.SeedPolicy{Enabled: true, Minutes: 2, Ratio: 1.5}
	if err = s.SetBTSettings(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if err = s.UseTorrentFiles(ctx, id, dest, false, true); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Seeding })
	if err = s.Execute(ctx, id, application.Pause); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Status == "paused" })
}

func TestBTSeedingStopsAtConfiguredMinute(t *testing.T) {
	s, _, dir := btService(t)
	ctx := t.Context()
	if err := s.SetBTSettings(ctx, application.BTSettings{DisableDHT: true, Seed: application.SeedPolicy{Enabled: true, Minutes: 1}}); err != nil {
		t.Fatal(err)
	}
	file, _ := btFixture(t)
	meta, err := s.PrepareTorrent(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.AddTorrent(ctx, meta.Hash, dir, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Seeding })
	started := time.Now()
	for time.Since(started) < 85*time.Second {
		if err = s.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		got := s.Snapshot().Tasks[0]
		if got.Status == "complete" {
			if time.Since(started) < 53*time.Second {
				t.Fatalf("seeding stopped prematurely after %v", time.Since(started))
			}
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("seeding did not stop at configured duration")
}

func TestBTArchivedTaskKeepsTorrentDetails(t *testing.T) {
	s, engine, dir := btService(t)
	file, _ := btFixture(t)
	ctx := t.Context()
	meta, err := s.PrepareTorrent(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.AddTorrent(ctx, meta.Hash, dir, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Status == "complete" })
	if err = engine.Client.Execute(ctx, id, application.Remove); err != nil {
		t.Fatal(err)
	}
	d, err := s.TorrentDetails(ctx, id)
	if err != nil || d.Pieces != 2 {
		t.Fatalf("archived: %+v, %v", d, err)
	}
}
