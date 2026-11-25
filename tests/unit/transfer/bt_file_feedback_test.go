package transfer_test

import (
	"context"
	"errors"
	"fmt"
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

func TestBTFileActionsIdentifyTheFileInBothLanguages(t *testing.T) {
	for _, tc := range []struct {
		code             application.ErrorCode
		chinese, english string
	}{
		{application.ErrTorrentFileMismatch, "文件 sample/folder/a.txt 的大小或类型不匹配", "File sample/folder/a.txt has an unexpected size or type"},
		{application.ErrTorrentFileMissing, "缺少文件 sample/folder/a.txt。请补齐后重试。", "File sample/folder/a.txt is missing. Restore it and try again."},
		{application.ErrTorrentMoveConflict, "请选择其他目录。当前目标目录已有 sample/folder/a.txt。", "Choose a different folder. The destination already contains sample/folder/a.txt."},
	} {
		t.Run(string(tc.code), func(t *testing.T) {
			api := delayedBTFileAPI{
				Service: service(t, &fakeEngine{}, &fakeRepo{}),
				snapshot: application.Snapshot{Tasks: []application.TaskDTO{{
					ID: "task", Name: "sample.torrent", TorrentHash: "hash", Status: "complete",
				}}},
				verifyErr: application.NamedError{Code: tc.code, Name: "sample/folder/a.txt"},
			}
			action := "Verify Files"
			if tc.code == application.ErrTorrentMoveConflict {
				action = "Move Files To…"
				api.started, api.finish = make(chan bool, 1), make(chan struct{})
				api.moveErr = api.verifyErr
				close(api.finish)
			}
			updates := make(chan func(), 2)
			dir := t.TempDir()
			view := presentation.New(api, presentation.Actions{
				Context: t.Context(), Dispatch: func(fn func()) { updates <- fn },
				PickDirectory: func(_ string, done func(string, error)) { done(dir, nil) },
			})
			view.Snapshot = api.Snapshot()
			scene := interactionScene(view, 800, 560)
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
			chooseSource(t, scene, action)
			applyUpdate()
			if !scene.HasText(tc.english) {
				t.Errorf("missing the affected file in English: %v", scene.Texts())
			}
			if tc.code == application.ErrTorrentMoveConflict {
				if bounds := sourceBounds(t, scene, ui.RoleText, tc.english); bounds.H < 25 {
					t.Errorf("the destination and recovery instructions are clipped: %v", bounds)
				}
			}
			scene.SetLocale("zh-CN")
			if !scene.HasText(tc.chinese) {
				t.Errorf("missing the affected file in Chinese: %v", scene.Texts())
			}
		})
	}
}

func TestRenderBTMoveFeedbackShowsAndScrollsTheOriginalFileList(t *testing.T) {
	var originals []string
	for i := range 2048 {
		originals = append(originals, fmt.Sprintf("set/sub/file-%04d.txt", i))
	}
	leftovers := make([]application.TorrentMoveOriginal, 0, len(originals))
	for _, name := range originals {
		leftovers = append(leftovers, application.TorrentMoveOriginal{Name: name, Cause: errors.New("private filesystem detail")})
	}
	leftovers[0].Cause = application.ErrTorrentFileChanged
	leftovers[1].Cause = application.ErrTorrentDestinationChanged
	api := delayedBTFileAPI{
		Service: service(t, &fakeEngine{}, &fakeRepo{}),
		started: make(chan bool, 1),
		finish:  make(chan struct{}),
		snapshot: application.Snapshot{Tasks: []application.TaskDTO{{
			ID: "task", Name: "sample.torrent", TorrentHash: "hash", Status: "complete",
		}}},
		moveErr: &application.TorrentMoveCleanupError{Originals: leftovers},
	}
	updates := make(chan func(), 2)
	var picked func(string, error)
	view := presentation.New(api, presentation.Actions{
		Context:       t.Context(),
		Dispatch:      func(fn func()) { updates <- fn },
		PickDirectory: func(_ string, done func(string, error)) { picked = done },
	})
	view.Snapshot = api.Snapshot()
	scene := interactionScene(view, 800, 560)
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
	const label = "Original files not removed"
	if !scene.HasText("The task uses the new folder. A destination file changed. Verify the new files before removing any originals.") {
		t.Errorf("missing the English move explanation: %v", scene.Texts())
	}
	list := sourceBounds(t, scene, ui.RoleList, label)
	if scene.HasText(originals[len(originals)/2]) || scene.HasText(originals[len(originals)-1]) {
		t.Error("the three-row viewport built offscreen file rows")
	}
	panel := sourceBounds(t, scene, ui.RoleDialog, "BitTorrent Details")
	first := sourceBounds(t, scene, ui.RoleText, originals[0])
	second := sourceBounds(t, scene, ui.RoleText, originals[1])
	for i, reason := range []string{"Original file changed", "Destination file changed", "Could not remove original"} {
		nameBounds := sourceBounds(t, scene, ui.RoleText, originals[i])
		reasonBounds := sourceBounds(t, scene, ui.RoleText, reason)
		if reasonBounds.Y < nameBounds.Y || reasonBounds.Y >= nameBounds.Y+nameBounds.H || reasonBounds.X <= nameBounds.X {
			t.Errorf("%q is not aligned with original %q: %v %v", reason, originals[i], reasonBounds, nameBounds)
		}
	}
	if scene.HasText("private filesystem detail") {
		t.Error("move feedback exposed raw filesystem details")
	}
	if panel.Y < 0 || list.Y < panel.Y || list.Y+list.H > panel.Y+panel.H || panel.Y+panel.H > 560 ||
		first.H <= 0 || first.Y < list.Y || second.Y <= first.Y || second.Y+second.H > list.Y+list.H {
		t.Fatalf("file rows are not visible inside the dialog: %v %v, list %v, dialog %v", first, second, list, panel)
	}
	for range 12 {
		node, _ := scene.AccessByRole(ui.RoleList, label)
		if node.Focused {
			break
		}
		scene.Key(0, ui.KeyTab)
	}
	if node, _ := scene.AccessByRole(ui.RoleList, label); !node.Focused {
		t.Fatal("file list is unreachable by keyboard")
	}
	scene.Key(0, ui.KeyEnd)
	last := sourceBounds(t, scene, ui.RoleText, originals[len(originals)-1])
	if last.H <= 0 || last.Y < list.Y || last.Y+last.H > list.Y+list.H {
		t.Errorf("keyboard scrolling did not reveal the last file: %v, list %v", last, list)
	}
	scene.SetLocale("zh-CN")
	if !scene.HasText("任务已切换到新目录。有目标文件已变化，请先校验新目录中的文件，再清理原文件。") {
		t.Errorf("missing the Chinese move explanation: %v", scene.Texts())
	}
	sourceBounds(t, scene, ui.RoleList, "未清理的原文件")
	scene.Key(0, ui.KeyHome)
	for _, reason := range []string{"原文件已变化", "目标文件已变化", "无法清理原文件"} {
		if !scene.HasText(reason) {
			t.Errorf("missing the Chinese cleanup reason %q: %v", reason, scene.Texts())
		}
	}
}
