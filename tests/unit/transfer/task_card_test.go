package transfer_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestTaskDetailsFollowFileIdentity(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	dir := t.TempDir()
	task := application.TaskDTO{ID: "a", Name: "manual.zip", URL: "https://example.org/docs/manual.zip", Directory: dir, Status: "paused"}
	v.Snapshot.Tasks = []application.TaskDTO{task}
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("查看详情 manual.zip"); err != nil {
		t.Fatal(err)
	}
	v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: "b", Name: "other.zip", Status: "waiting"})
	tt.Frame()
	if !tt.HasText(task.URL) || !tt.HasText(dir) {
		t.Fatal("source and destination missing after insertion")
	}
	if err := tt.Click("返回任务列表"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText(task.URL) {
		t.Fatal("detail did not collapse")
	}
}

func TestCardFailureActionsFitMinimumWindow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.Snapshot.Tasks = []application.TaskDTO{{ID: "f", Name: strings.Repeat("很长的文件名", 20) + ".zip", Status: "error", Failure: application.FailureSourceUnavailable, CanRestart: true, CanRetry: true}}
		tt := ui.NewTesterWith(v.Render, 800, 528, ui.TesterOptions{Dark: dark, Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true}})
		for _, label := range []string{"查看详情 " + v.Snapshot.Tasks[0].Name, "无法访问下载来源，请重试", "从头下载", "移除任务（保留文件）"} {
			r, ok := tt.Find(label)
			if !ok || r.W <= 0 || r.X < 0 || r.X+r.W > 800 || r.Y+r.H > 528 {
				t.Errorf("dark=%v %s outside window: %+v", dark, label, r)
			}
		}
	}
}

func TestCardsOpenDetailsAndComposeFromTheDock(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one"}, {ID: "b", Name: "two"}}
	tt := interactionScene(v, 1000, 640)
	a, _ := tt.Find("查看详情 one")
	b, _ := tt.Find("查看详情 two")
	if a.Y != b.Y || a.X == b.X {
		t.Errorf("tasks do not share a grid row: %v, %v", a, b)
	}
	dock, ok := tt.Find("新建下载")
	if !ok || dock.Y < 500 {
		t.Errorf("compose control not at bottom: %+v", dock)
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	panel, ok := tt.AccessByRole(ui.RoleDialog, "")
	if !ok || panel.Bounds.Y+panel.Bounds.H < 610 {
		t.Errorf("entry form not anchored to the dock: %v", panel.Bounds)
	}
}

func TestRetryFailureIsReportedOnceOnItsCard(t *testing.T) {
	f := &fakeEngine{}
	s := service(t, f, &fakeRepo{})
	_, err := s.Add(context.Background(), "https://example.org/a", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.observed = []application.Observation{{ID: s.Snapshot().Tasks[0].ID, Status: "error", Error: "previous failure"}}
	if err = s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.addErr = application.ErrEngineConnection
	updates := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { updates <- fn }})
	v.Snapshot = s.Snapshot()
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("重试下载"); err != nil {
		t.Fatal(err)
	}
	select {
	case fn := <-updates:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("retry did not finish")
	}
	tt.Frame()
	n := 0
	for _, label := range tt.Texts() {
		if label == "无法连接下载引擎" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("rendered %d copies of the same task failure", n)
	}
}

func TestWorkspaceRemoveSaveFailureStaysInTaskList(t *testing.T) {
	repo := &fakeRepo{}
	s := service(t, &fakeEngine{}, repo)
	if _, err := s.Add(context.Background(), "https://example.org/one.zip", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	repo.saveErr = errors.New("disk full")
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { done <- fn }})
	v.Snapshot = s.Snapshot()
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("移除任务（保留文件）"); err != nil {
		t.Fatal(err)
	}
	select {
	case fn := <-done:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("remove did not finish")
	}
	tt.Frame()
	if !tt.HasText("保存任务失败") {
		t.Fatal("removing the card hid the persistence failure")
	}
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("保存任务失败") {
		t.Fatal("task list failure escaped into settings")
	}
}

func TestFailedCardMakesRecoveryVisible(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "failed", Name: "__down", Status: "error", Error: "下载引擎 HTTP 状态 400", CanRetry: true, CanRestart: true}}
	tt := interactionScene(v, 800, 528)
	action, ok := tt.Find("重试下载")
	if !ok || action.W < 72 || !slices.Contains(tt.Texts(), "下载失败") {
		t.Fatalf("recovery action area %v, present %v: %v", action, ok, tt.Texts())
	}
}

func TestRenderTransferFlowTracksActivityAndMotionPreference(t *testing.T) {
	for _, tc := range []struct {
		name, status   string
		total          int64
		reduced, moves bool
	}{
		{"downloading", "active", 1024, false, true}, {"unknown size", "active", 0, false, true},
		{"paused", "paused", 1024, false, false}, {"reduced motion", "active", 1024, true, false},
		{"unknown size reduced motion", "active", 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := presentation.New(nil, presentation.Actions{})
			v.Ready = true
			v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one.zip", Status: tc.status, Total: tc.total, Completed: 512, Progress: .5, Speed: 128}}
			tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: tc.reduced}})
			tt.Advance(time.Second)
			n, ok := tt.AccessByRole(ui.RoleProgress, "下载进度")
			if !ok {
				t.Fatal("no visible transfer indicator")
			}
			r := n.Bounds
			strip := func() []byte {
				im := tt.Image()
				y := int(r.Y + r.H/2)
				return slices.Clone(im.Pix[im.PixOffset(int(r.X)+2, y):im.PixOffset(int(r.X+r.W)-2, y)])
			}
			first := strip()
			tt.Advance(450 * time.Millisecond)
			second := strip()
			tt.Advance(450 * time.Millisecond)
			changes := !slices.Equal(first, second) || !slices.Equal(second, strip())
			if changes != tc.moves {
				t.Errorf("flow animation=%v, want %v", changes, tc.moves)
			}
		})
	}
}

func TestPrimaryTaskControlsKeepTheirGeometryAcrossStates(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	statusLabelForTest := map[string]string{"active": "下载中", "paused": "已暂停", "waiting": "等待中", "error": "下载失败", "complete": "已完成"}
	var baseline, trackBaseline ui.Rect
	for i, tc := range []struct{ status, label string }{{"active", "暂停下载"}, {"paused", "继续下载"}, {"error", "重试下载"}, {"complete", "查看文件详情"}, {"waiting", "暂停下载"}} {
		task := application.TaskDTO{ID: "a", Name: "a.zip", Status: tc.status, Total: 1024, Completed: 512, Progress: .5, CanPause: i == 0 || tc.status == "waiting", CanResume: i == 1, CanRetry: i == 2, CanRestart: i == 2}
		v.Snapshot.Tasks = []application.TaskDTO{task}
		tt := interactionScene(v, 900, 600)
		b, ok := tt.AccessByRole(ui.RoleButton, tc.label)
		if !ok {
			t.Fatal(tc.label)
		}
		if !strings.Contains(b.Description, statusLabelForTest[tc.status]) || !tt.HasText(statusLabelForTest[tc.status]) {
			t.Errorf("%s: missing state on primary button: %v", tc.status, b.Description)
		}
		progress, hasTrack := tt.AccessByRole(ui.RoleProgress, "下载进度")
		if i == 0 {
			trackBaseline = progress.Bounds
		} else if tc.status == "complete" {
			if hasTrack {
				t.Error("completed task still shows a track")
			}
		} else if progress.Bounds != trackBaseline {
			t.Errorf("%s track moved %v -> %v", tc.status, trackBaseline, progress.Bounds)
		}
		if _, ok := tt.Find("从头下载"); ok != task.CanRestart {
			t.Errorf("%s restart visibility %v", tc.status, ok)
		}
		if _, ok := tt.Find("在文件夹中显示"); ok {
			t.Error("reveal shown without a file")
		}
		card, _ := tt.Find("a.zip")
		remove, ok := tt.Find("移除任务（保留文件）")
		if !ok || remove.Y > card.Y+20 || remove.X+remove.W < card.X+card.W-20 || slices.Contains(tt.Texts(), "移除") {
			t.Errorf("remove must be an icon in upper right: %v %v", card, remove)
		}
		if i == 0 {
			baseline = b.Bounds
		} else if baseline != b.Bounds {
			t.Errorf("%s: control moved %v -> %v", tc.status, baseline, b.Bounds)
		}
	}
}

func TestRenderPauseInterpolatesTheExistingProgressTrack(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "same", Name: "sample.zip", Status: "active", CanPause: true, Total: 1024, Completed: 512, Progress: .5}}
	tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	bar, _ := tt.AccessByRole(ui.RoleProgress, "下载进度")
	x, y := int(bar.Bounds.X+10), int(bar.Bounds.Y+bar.Bounds.H/2)
	start := tt.Image().RGBAAt(x, y)
	task := &v.Snapshot.Tasks[0]
	task.Status, task.CanPause, task.CanResume = "paused", false, true
	tt.Frame()
	tt.Advance(90 * time.Millisecond)
	mid := tt.Image().RGBAAt(x, y)
	tt.Advance(time.Second)
	end := tt.Image().RGBAAt(x, y)
	if mid == start || mid == end || start == end {
		t.Fatalf("missing intermediate track color: %v %v %v", start, mid, end)
	}
	paused, _ := tt.AccessByRole(ui.RoleProgress, "下载进度")
	if paused.Bounds != bar.Bounds {
		t.Fatalf("track shifted: %v -> %v", bar.Bounds, paused.Bounds)
	}
}

func TestRenderCardMaterialFollowsTransferStatus(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "same", Name: "clip.wav", Status: "paused", CanResume: true}}
	tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	tt.Advance(time.Second)
	head, _ := tt.Find("查看详情 clip.wav")
	x, y := int(head.X+2), int(head.Y+3)
	first := tt.Image().RGBAAt(x, y)
	v.Snapshot.Tasks[0].Name = "clip.zip"
	tt.Frame()
	tt.Advance(time.Second)
	if next := tt.Image().RGBAAt(x, y); first != next {
		t.Errorf("type changed card material: %v -> %v", first, next)
	}
	v.Snapshot.Tasks[0].Status = "error"
	tt.Frame()
	tt.Advance(time.Second)
	if next := tt.Image().RGBAAt(x, y); first == next {
		t.Errorf("failure did not change the card material: %v", next)
	}
}
