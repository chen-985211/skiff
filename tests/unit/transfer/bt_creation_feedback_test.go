package transfer_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

type torrentCreationEngine struct {
	fakeEngine
	writes int
}

func (e *torrentCreationEngine) WriteTorrent(context.Context, string, string, []string) (application.Torrent, error) {
	e.writes++
	return e.CachedTorrent("fixture")
}

func (e *torrentCreationEngine) PrepareTorrent(context.Context, string) (application.Torrent, error) {
	return e.CachedTorrent("fixture")
}

func (*torrentCreationEngine) CachedTorrent(string) (application.Torrent, error) {
	return application.Torrent{Hash: "fixture", Name: "sample.txt", Files: []application.TorrentFile{{Path: "sample.txt", Size: 1}}}, nil
}

func torrentCreationScene(t *testing.T, engine *torrentCreationEngine, existing bool) (*ui.Tester, <-chan func()) {
	t.Helper()
	dir := t.TempDir()
	svc := application.New(engine, &fakeRepo{}, dir, func() string { return "task" })
	if existing {
		if _, err := svc.AddTorrent(t.Context(), "fixture", dir, []int{1}); err != nil {
			t.Fatal(err)
		}
	}
	updates := make(chan func(), 1)
	view := presentation.New(svc, presentation.Actions{
		Context:    t.Context(),
		Dispatch:   func(fn func()) { updates <- fn },
		PickSource: func(_ bool, done func(string, error)) { done(filepath.Join(dir, "sample.txt"), nil) },
		SaveTorrent: func(_ string, done func(string, error)) {
			done(filepath.Join(dir, "saved.torrent"), nil)
		},
	})
	view.Snapshot = svc.Snapshot()
	view.OpenNew()
	scene := interactionScene(view, 900, 600)
	scene.SetLocale("en")
	for _, label := range []string{"BT", "BitTorrent options", "Create Torrent…", "Choose torrent source", "File…"} {
		chooseSource(t, scene, label)
	}
	return scene, updates
}

func submitTorrentCreation(t *testing.T, scene *ui.Tester, updates <-chan func()) {
	t.Helper()
	chooseSource(t, scene, "Save Torrent…")
	select {
	case update := <-updates:
		update()
		scene.Frame()
	case <-time.After(time.Second):
		t.Fatal("torrent creation did not finish")
	}
}

func TestTorrentCreationIdentifiesTheInvalidTrackerEntry(t *testing.T) {
	engine := &torrentCreationEngine{}
	scene, updates := torrentCreationScene(t, engine, false)
	chooseSource(t, scene, "Torrent trackers")
	scene.Type("https://tracker.test/announce\n\ninvalid://tracker.test/private-passkey")
	submitTorrentCreation(t, scene, updates)
	if engine.writes != 0 || !scene.HasText("Tracker address 2 is invalid.") || scene.HasText("invalid://tracker.test/private-passkey") {
		t.Errorf("invalid tracker was not identified without its secret: %v", scene.Texts())
	}
	scene.SetLocale("zh-CN")
	if !scene.HasText("第 2 个 Tracker 地址无效。") {
		t.Errorf("missing the invalid tracker entry in Chinese: %v", scene.Texts())
	}
}
