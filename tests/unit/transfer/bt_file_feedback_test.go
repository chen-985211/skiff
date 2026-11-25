package transfer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

type delayedBTFileAPI struct {
	*application.Service
	started   chan bool
	finish    chan struct{}
	snapshot  application.Snapshot
	verifyErr error
	moveErr   error
}

func (a delayedBTFileAPI) Snapshot() application.Snapshot { return a.snapshot }

func (a delayedBTFileAPI) TorrentDetails(context.Context, string) (application.BTDetails, error) {
	return application.BTDetails{}, nil
}

func (a delayedBTFileAPI) VerifyTorrentFiles(context.Context, string) error {
	return a.verifyErr
}

func (a delayedBTFileAPI) UseTorrentFiles(ctx context.Context, _ string, _ string, move, _ bool) error {
	a.started <- move
	select {
	case <-a.finish:
		return a.moveErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestBTFilePickerFeedbackUsesRenderedLanguage(t *testing.T) {
	for _, tc := range []struct {
		option string
		move   bool
	}{{"Find Existing Files…", false}, {"Move Files To…", true}} {
		t.Run(tc.option, func(t *testing.T) {
			api := delayedBTFileAPI{
				Service: service(t, &fakeEngine{}, &fakeRepo{}),
				started: make(chan bool, 1),
				finish:  make(chan struct{}),
				snapshot: application.Snapshot{Tasks: []application.TaskDTO{{
					ID: "task", Name: "sample.torrent", TorrentHash: "hash", Status: "complete",
				}}},
			}
			updates := make(chan func(), 2)
			var picked func(string, error)
			v := presentation.New(api, presentation.Actions{
				Context:       t.Context(),
				Dispatch:      func(fn func()) { updates <- fn },
				PickDirectory: func(_ string, done func(string, error)) { picked = done },
			})
			v.Snapshot = api.Snapshot()
			scene := interactionScene(v, 1020, 680)
			scene.SetLocale("en")
			applyUpdate := func() {
				t.Helper()
				select {
				case update := <-updates:
					update()
					scene.Frame()
				case <-time.After(time.Second):
					t.Fatal("no asynchronous update")
				}
			}
			chooseSource(t, scene, "View details for sample.torrent")
			chooseSource(t, scene, "BitTorrent Details")
			applyUpdate()
			chooseSource(t, scene, "Torrent file actions")
			chooseSource(t, scene, tc.option)
			if picked == nil {
				t.Fatal("file action did not invoke the directory picker")
			}
			// The desktop delivers this callback between MyGo render frames.
			picked(t.TempDir(), nil)
			select {
			case move := <-api.started:
				if move != tc.move {
					t.Errorf("move = %v, want %v", move, tc.move)
				}
			case <-time.After(time.Second):
				t.Fatal("file action did not start")
			}
			scene.Frame()
			if !scene.HasText("Working on files…") {
				t.Errorf("missing English working feedback: %v", scene.Texts())
			}
			close(api.finish)
			applyUpdate()
			if !scene.HasText("Files verified and location updated") {
				t.Errorf("missing English completion feedback: %v", scene.Texts())
			}
			scene.SetLocale("zh-CN")
			if !scene.HasText("文件已校验，位置已更新") {
				t.Errorf("existing completion feedback did not switch language: %v", scene.Texts())
			}
		})
	}
}

func TestInvalidTorrentPeerLimitShowsTheAllowedRange(t *testing.T) {
	api := delayedBTFileAPI{
		Service: service(t, &fakeEngine{}, &fakeRepo{}),
		snapshot: application.Snapshot{Tasks: []application.TaskDTO{{
			ID: "task", Name: "sample.torrent", TorrentHash: "hash", Status: "complete",
		}}},
	}
	updates := make(chan func(), 2)
	view := presentation.New(api, presentation.Actions{
		Context: t.Context(), Dispatch: func(fn func()) { updates <- fn },
	})
	view.Snapshot = api.Snapshot()
	scene := interactionScene(view, 1020, 680)
	scene.SetLocale("en")
	chooseSource(t, scene, "View details for sample.torrent")
	chooseSource(t, scene, "BitTorrent Details")
	select {
	case update := <-updates:
		update()
		scene.Frame()
	case <-time.After(time.Second):
		t.Fatal("no inspector update")
	}
	for _, input := range []string{"abc", "999999999999999999999"} {
		chooseSource(t, scene, "Peer limit for this torrent")
		scene.Key(ui.Cmd, ui.KeyA)
		scene.Type(input)
		chooseSource(t, scene, "Apply")
		if !scene.HasText("Enter a peer limit from 0 to 65535") {
			t.Errorf("missing the peer limit range for %q: %v", input, scene.Texts())
		}
	}
}

func TestBTFileVerificationIdentifiesTheFileInBothLanguages(t *testing.T) {
	for _, tc := range []struct {
		code             application.ErrorCode
		chinese, english string
	}{
		{application.ErrTorrentFileMismatch, "文件 sample/folder/a.txt 的大小或类型不匹配", "File sample/folder/a.txt has an unexpected size or type"},
		{application.ErrTorrentFileMissing, "缺少文件 sample/folder/a.txt。请补齐后重试。", "File sample/folder/a.txt is missing. Restore it and try again."},
	} {
		t.Run(string(tc.code), func(t *testing.T) {
			api := delayedBTFileAPI{
				Service: service(t, &fakeEngine{}, &fakeRepo{}),
				snapshot: application.Snapshot{Tasks: []application.TaskDTO{{
					ID: "task", Name: "sample.torrent", TorrentHash: "hash", Status: "complete",
				}}},
				verifyErr: application.NamedError{Code: tc.code, Name: "sample/folder/a.txt"},
			}
			updates := make(chan func(), 2)
			view := presentation.New(api, presentation.Actions{
				Context: t.Context(), Dispatch: func(fn func()) { updates <- fn },
			})
			view.Snapshot = api.Snapshot()
			scene := interactionScene(view, 1020, 680)
			scene.SetLocale("en")
			applyUpdate := func() {
				t.Helper()
				select {
				case update := <-updates:
					update()
					scene.Frame()
				case <-time.After(time.Second):
					t.Fatal("no inspector update")
				}
			}
			chooseSource(t, scene, "View details for sample.torrent")
			chooseSource(t, scene, "BitTorrent Details")
			applyUpdate()
			chooseSource(t, scene, "Torrent file actions")
			chooseSource(t, scene, "Verify Files")
			applyUpdate()
			if !scene.HasText(tc.english) {
				t.Errorf("missing the affected file in English: %v", scene.Texts())
			}
			scene.SetLocale("zh-CN")
			if !scene.HasText(tc.chinese) {
				t.Errorf("missing the affected file in Chinese: %v", scene.Texts())
			}
		})
	}
}

func TestBTMoveFeedbackExplainsThatTheNewFolderIsAlreadyInUse(t *testing.T) {
	api := delayedBTFileAPI{
		Service: service(t, &fakeEngine{}, &fakeRepo{}),
		started: make(chan bool, 1),
		finish:  make(chan struct{}),
		snapshot: application.Snapshot{Tasks: []application.TaskDTO{{
			ID: "task", Name: "sample.torrent", TorrentHash: "hash", Status: "complete",
		}}},
		moveErr: &application.TorrentMoveCleanupError{
			Originals: []string{"set/a.txt", "set/sub/b.txt"},
			Cause:     errors.New("private transport or filesystem detail"),
		},
	}
	updates := make(chan func(), 2)
	var picked func(string, error)
	view := presentation.New(api, presentation.Actions{
		Context:       t.Context(),
		Dispatch:      func(fn func()) { updates <- fn },
		PickDirectory: func(_ string, done func(string, error)) { picked = done },
	})
	view.Snapshot = api.Snapshot()
	scene := interactionScene(view, 1020, 680)
	scene.SetLocale("en")
	applyUpdate := func() {
		t.Helper()
		select {
		case update := <-updates:
			update()
			scene.Frame()
		case <-time.After(time.Second):
			t.Fatal("no inspector update")
		}
	}
	chooseSource(t, scene, "View details for sample.torrent")
	chooseSource(t, scene, "BitTorrent Details")
	applyUpdate()
	chooseSource(t, scene, "Torrent file actions")
	chooseSource(t, scene, "Move Files To…")
	if picked == nil {
		t.Fatal("move did not ask for a folder")
	}
	picked(t.TempDir(), nil)
	close(api.finish)
	applyUpdate()
	english := "The task now uses the new folder. Skiff did not remove these original files:\nset/a.txt\nset/sub/b.txt"
	if !scene.HasText(english) {
		t.Errorf("missing full English move feedback: %v", scene.Texts())
	}
	scene.SetLocale("zh-CN")
	chinese := "任务已切换到新目录。Skiff 未清理以下原文件：\nset/a.txt\nset/sub/b.txt"
	if !scene.HasText(chinese) {
		t.Errorf("missing full Chinese move feedback: %v", scene.Texts())
	}
}
