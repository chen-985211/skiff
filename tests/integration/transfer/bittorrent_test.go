//go:build integration

package transfer_test

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
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

	"github.com/anacrolix/torrent/metainfo"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"skiff/internal/platform/diagnostics"
)

func btService(t *testing.T) (*application.Service, *aria2.Process, string) {
	t.Helper()
	return btServiceWithRepository(t, nil)
}

func btServiceWithRepository(t *testing.T, wrap func(application.Repository) application.Repository) (*application.Service, *aria2.Process, string) {
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
	var repo application.Repository = persistence.Repository{Path: filepath.Join(data, "tasks.json")}
	if wrap != nil {
		repo = wrap(repo)
	}
	s := application.New(p.Client, repo, dir, func() string { seq++; return fmt.Sprintf("%016x", seq) })
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
			if r.URL.Path == "/empty" {
				fmt.Fprintln(w, "no trackers")
			} else {
				http.Error(w, "offline", 503)
			}
			return
		}
		fmt.Fprintln(w, "udp://127.0.0.1:9090/announce\n\nhttp://127.0.0.1:7070/announce\nudp://127.0.0.1:9090/announce")
	}))
	defer server.Close()
	cfg := application.BTSettings{Sources: server.URL + "/empty\n" + server.URL, MaxPeers: 12, ListenPort: 9870}
	if err := s.SetBTSettings(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	observed := diagnostics.New(&output).ObserveTransfer(s)
	if err := observed.RefreshBTSubscriptions(t.Context(), true); err != nil || output.Len() > 0 {
		t.Fatalf("successful refresh returned %v and logged %s", err, output.String())
	}
	sub := s.Snapshot().BTSubscriptions
	if len(sub) != 2 || len(sub[0].Trackers) != 2 || sub[0].Failure != "" || sub[1].Failure != "" {
		t.Fatal(sub)
	}
	broken.Store(true)
	err := observed.RefreshBTSubscriptions(t.Context(), true)
	if !errors.Is(err, application.ErrReadSubscription) || !errors.Is(err, application.ErrEmptySubscription) {
		t.Fatalf("refresh did not return both subscription errors: %v", err)
	}
	sub = s.Snapshot().BTSubscriptions
	if len(sub[0].Trackers) != 2 || len(sub[1].Trackers) != 2 || sub[0].Failure != application.ErrEmptySubscription || sub[1].Failure != application.ErrReadSubscription {
		t.Fatal("lost cached trackers or health", sub)
	}
	if size := output.Len(); observed.RefreshBTSubscriptions(t.Context(), false) != nil || output.Len() != size {
		t.Fatal("background refresh retried or logged fresh manual failure state", output.String())
	}
	if strings.Contains(output.String(), server.URL) {
		t.Fatal("tracker URL leaked into diagnostics", output.String())
	}
	decoder := json.NewDecoder(&output)
	var correlationID string
	for _, tc := range []struct {
		code  application.ErrorCode
		level string
	}{{application.ErrEmptySubscription, "warn"}, {application.ErrReadSubscription, "error"}} {
		var entry struct {
			Level, Scope, Operation, CorrelationID string
			Error                                  struct{ Code application.ErrorCode }
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("missing diagnostic for %s: %v", tc.code, err)
		}
		if entry.Level != tc.level || entry.Error.Code != tc.code || entry.CorrelationID == "" ||
			entry.Scope != "transfer.tracker" || entry.Operation != "refresh_subscriptions" ||
			(correlationID != "" && correlationID != entry.CorrelationID) {
			t.Fatalf("invalid diagnostic for %s: %+v", tc.code, entry)
		}
		correlationID = entry.CorrelationID
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
	if err = os.WriteFile(file, []byte("short"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.VerifyTorrentFiles(ctx, id); !errors.Is(err, application.ErrTorrentFileMismatch) || !strings.Contains(err.Error(), "set/a.txt") {
		t.Fatalf("payload mismatch should identify its relative file: %v", err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	err = s.VerifyTorrentFiles(ctx, id)
	var missing application.NamedError
	if !errors.Is(err, os.ErrNotExist) || application.CodeOf(err) != application.ErrTorrentFileMissing ||
		!errors.As(err, &missing) || missing.Name != "set/a.txt" {
		t.Fatalf("missing payload should retain its code, relative name and OS cause: %v", err)
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

func TestBTMoveNamesTheConflictingDestinationFile(t *testing.T) {
	svc, _, origin := btService(t)
	ctx := t.Context()
	source, _ := btFixture(t)
	meta, err := svc.PrepareTorrent(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.AddTorrent(ctx, meta.Hash, origin, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Status == "complete" })
	destination := t.TempDir()
	const conflict = "set/sub/b.txt"
	path := filepath.Join(destination, filepath.FromSlash(conflict))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("keep existing file"), 0600); err != nil {
		t.Fatal(err)
	}
	err = svc.UseTorrentFiles(ctx, id, destination, true, false)
	var named application.NamedError
	if application.CodeOf(err) != application.ErrTorrentMoveConflict || !errors.As(err, &named) || named.Name != conflict || !errors.Is(err, os.ErrExist) {
		t.Errorf("move conflict lost its stable code, relative name or OS cause: %v", err)
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != "keep existing file" {
		t.Errorf("move did not preserve the existing file: %q, %v", contents, err)
	}
	if task := svc.Snapshot().Tasks[0]; task.Directory != origin {
		t.Errorf("failed move changed the saved location: %q", task.Directory)
	}
	if _, err := os.Stat(filepath.Join(destination, "set", "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("failed move kept an uncommitted copy: %v", err)
	}
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

func TestBTCreateV1FileAndDirectoryAndSeedOriginal(t *testing.T) {
	for _, folder := range []bool{false, true} {
		t.Run(fmt.Sprintf("directory=%v", folder), func(t *testing.T) {
			// The seeder must exit before TempDir removes its open payload.
			parent := t.TempDir()
			s, _, _ := btService(t)
			ctx := t.Context()
			source := filepath.Join(parent, "sample.bin")
			a := []byte(strings.Repeat("a", 175001))
			b := []byte(strings.Repeat("b", 65003))
			data := a
			if folder {
				source = filepath.Join(parent, "samples")
				if err := os.MkdirAll(filepath.Join(source, "sub"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "a.bin"), a, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "sub", "b.bin"), b, 0600); err != nil {
					t.Fatal(err)
				}
				data = append(append([]byte{}, a...), b...)
			} else if err := os.WriteFile(source, a, 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "export.torrent")
			manifest, err := s.CreateTorrent(ctx, source, output, []string{"http://127.0.0.1:1/announce"}, true)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := metainfo.LoadFromFile(output)
			if err != nil {
				t.Fatal(err)
			}
			info, err := raw.UnmarshalInfo()
			if err != nil {
				t.Fatal(err)
			}
			if info.MetaVersion != 0 || info.PieceLength < 1 || info.TotalLength() != int64(len(data)) || raw.HashInfoBytes().HexString() != manifest.Hash || info.IsDir() != folder || raw.Announce != "http://127.0.0.1:1/announce" {
				t.Fatal(info, manifest)
			}
			var hashes []byte
			for start := int64(0); start < int64(len(data)); start += info.PieceLength {
				sum := sha1.Sum(data[start:min(start+info.PieceLength, int64(len(data)))])
				hashes = append(hashes, sum[:]...)
			}
			if !bytes.Equal(hashes, info.Pieces) {
				t.Fatal("v1 piece hashes differ from original bytes")
			}
			var id string
			for _, task := range s.Snapshot().Tasks {
				if task.TorrentHash == manifest.Hash {
					id = task.ID
					if task.Directory != parent {
						t.Fatal("seeding a copy")
					}
				}
			}
			if id == "" {
				t.Fatal("no task for original files")
			}
			awaitTask(t, s, id, func(task application.TaskDTO) bool { return task.Seeding })
			if err = s.Execute(ctx, id, application.Pause); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestBTCreateRejectsInputAsOutput(t *testing.T) {
	s, _, _ := btService(t)
	path := filepath.Join(t.TempDir(), "original.torrent")
	if err := os.WriteFile(path, []byte("original bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTorrent(t.Context(), path, path, nil, false); application.CodeOf(err) != application.ErrTorrentDestinationIsSource {
		t.Fatalf("expected a stable unsafe-destination error, got %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "original bytes" {
		t.Fatalf("%s %v", b, err)
	}
}

func TestBTCreatedSeedServesPeerAndStopsAtShareRatio(t *testing.T) {
	seed, _, _ := btService(t)
	peer, _, dest := btService(t)
	ctx := t.Context()
	if err := seed.SetBTSettings(ctx, application.BTSettings{DisableDHT: true, Seed: application.SeedPolicy{Enabled: true, Ratio: 1}}); err != nil {
		t.Fatal(err)
	}
	var seederPort atomic.Int32
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		port, _ := strconv.Atoi(r.URL.Query().Get("port"))
		if r.URL.Query().Get("left") == "0" {
			seederPort.CompareAndSwap(0, int32(port))
		}
		var peers []byte
		if p := int(seederPort.Load()); p != 0 && p != port {
			peers = []byte{127, 0, 0, 1, byte(p >> 8), byte(p)}
		}
		fmt.Fprintf(w, "d8:intervali1e5:peers%d:%se", len(peers), peers)
	}))
	defer tracker.Close()
	source := filepath.Join(t.TempDir(), "local.bin")
	data := bytes.Repeat([]byte("local peer payload"), 16000)
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	torrent := filepath.Join(t.TempDir(), "local.torrent")
	meta, err := seed.CreateTorrent(ctx, source, torrent, []string{tracker.URL}, true)
	if err != nil {
		t.Fatal(err)
	}
	seedID := seed.Snapshot().Tasks[0].ID
	awaitTask(t, seed, seedID, func(task application.TaskDTO) bool { return task.Seeding && seederPort.Load() > 0 })
	if _, err = peer.PrepareTorrent(ctx, torrent); err != nil {
		t.Fatal(err)
	}
	peerID, err := peer.AddTorrent(ctx, meta.Hash, dest, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	awaitTask(t, peer, peerID, func(task application.TaskDTO) bool { return task.Status == "complete" })
	actual, err := os.ReadFile(filepath.Join(dest, "local.bin"))
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatalf("peer bytes mismatch: %v", err)
	}
	awaitTask(t, seed, seedID, func(task application.TaskDTO) bool { return task.Status == "complete" })
	d, err := seed.TorrentDetails(ctx, seedID)
	if err != nil || d.Uploaded < int64(len(data)) {
		t.Fatalf("seed stopped without uploading ratio: %v %+v", err, d)
	}
}

func TestBTCreateCancellationKeepsSourceAndSkipsOutput(t *testing.T) {
	s, _, _ := btService(t)
	source := filepath.Join(t.TempDir(), "source.bin")
	out := source + ".torrent"
	if err := os.WriteFile(source, []byte("leave whole"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.CreateTorrent(ctx, source, out, nil, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("published cancelled output", err)
	}
	restored, err := os.ReadFile(source)
	if err != nil || string(restored) != "leave whole" {
		t.Fatal("source was touched")
	}
}

type btMoveRepository struct {
	application.Repository
	afterSave func(application.State)
}

func (r *btMoveRepository) Save(ctx context.Context, state application.State) error {
	if err := r.Repository.Save(ctx, state); err != nil {
		return err
	}
	if r.afterSave != nil {
		r.afterSave(state)
	}
	return nil
}

func TestBTMoveReportsEveryOriginalThatRemainsAfterCommit(t *testing.T) {
	repo := &btMoveRepository{}
	svc, _, origin := btServiceWithRepository(t, func(base application.Repository) application.Repository {
		repo.Repository = base
		return repo
	})
	ctx := t.Context()
	source, _ := btFixture(t)
	meta, err := svc.PrepareTorrent(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.AddTorrent(ctx, meta.Hash, origin, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, id, func(task application.TaskDTO) bool { return task.Status == "complete" })

	dest := t.TempDir()
	const first = "set/a.txt"
	const second = "set/sub/b.txt"
	repo.afterSave = func(state application.State) {
		if state.Tasks[0].Directory != dest {
			return
		}
		// Change different sides after the queue commits and before original cleanup.
		for _, name := range []string{filepath.Join(origin, first), filepath.Join(dest, second)} {
			if err := os.WriteFile(name, []byte("changed"), 0600); err != nil {
				t.Error(err)
			}
		}
	}
	err = svc.UseTorrentFiles(ctx, id, dest, true, false)
	if application.CodeOf(err) != application.ErrTorrentMoveCleanup || !errors.Is(err, application.ErrTorrentFileChanged) || !errors.Is(err, application.ErrTorrentDestinationChanged) {
		t.Errorf("move did not distinguish committed cleanup: %v", err)
	}
	var cleanup *application.TorrentMoveCleanupError
	if !errors.As(err, &cleanup) || len(cleanup.Originals) != 2 {
		t.Fatalf("move did not report both original files: %+v", cleanup)
	}
	if original := cleanup.Originals[0]; original.Name != first || application.CodeOf(original.Cause) != application.ErrTorrentFileChanged {
		t.Errorf("move did not report why %q was preserved: %+v", first, original)
	}
	if original := cleanup.Originals[1]; original.Name != second || application.CodeOf(original.Cause) != application.ErrTorrentDestinationChanged {
		t.Errorf("move did not report why %q was preserved: %+v", second, original)
	}
	if got := svc.Snapshot().Tasks[0].Directory; got != dest {
		t.Errorf("task directory = %q, want %q", got, dest)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(filepath.Join(origin, path)); err != nil {
			t.Errorf("original %q was not preserved: %v", path, err)
		}
	}
}
