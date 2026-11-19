package transfer_test

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestRenderDownloadReachesFullBeforeCompletion(t *testing.T) {
	for _, mode := range []string{"card", "detail", "active filter"} {
		t.Run(mode, func(t *testing.T) {
			v := presentation.New(nil, presentation.Actions{})
			v.Ready = true
			v.Snapshot.Tasks = []application.TaskDTO{{ID: "photo", Name: "photo.jpg", Status: "active", CanPause: true, Total: 100, Completed: 60, Progress: .6}}
			if mode == "active filter" {
				v.Filter = "active"
			}
			scene := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
			if mode == "detail" {
				if err := scene.Click("查看详情 photo.jpg"); err != nil {
					t.Fatal(err)
				}
			}
			scene.Advance(time.Second)
			task := &v.Snapshot.Tasks[0]
			task.Status, task.CanPause, task.Completed, task.Progress = "complete", false, 100, 1
			scene.Frame()
			if !scene.HasText("60%") || !scene.HasText("下载中") {
				t.Fatalf("completion cut off the last visible progress: %v", scene.Texts())
			}
			if _, ok := scene.AccessByRole(ui.RoleButton, "暂停下载"); ok {
				t.Fatal("a finished download still offers pause")
			}
			scene.Advance(400 * time.Millisecond)
			track, ok := scene.AccessByRole(ui.RoleProgress, "下载进度")
			if !ok || track.Now <= .70 || track.Now >= .90 || !scene.HasText(fmt.Sprintf("%.0f%%", 100*track.Now)) {
				t.Fatalf("halfway through the fill, the bar and counter should both be near 80%%: %v, %v", track, scene.Texts())
			}
			scene.Advance(450 * time.Millisecond)
			if !scene.HasText("100%") || !scene.HasText("下载中") {
				t.Fatalf("full progress must be readable before the status changes: %v", scene.Texts())
			}
			sample := func() uint8 {
				return scene.Image().RGBAAt(int(track.Bounds.X+track.Bounds.W*.85), int(track.Bounds.Y+track.Bounds.H/2)).R
			}
			full := sample()
			scene.Advance(530 * time.Millisecond)
			if !scene.HasText("100%") || !scene.HasText("下载中") || sample() != full {
				t.Fatal("the full bar should remain steady until the 600 ms hold ends")
			}
			scene.Advance(60 * time.Millisecond)
			if scene.HasText("下载中") || !scene.HasText("100%") {
				t.Fatal("the full bar should be fading into the completed state after 600 ms")
			}
			scene.Advance(185 * time.Millisecond)
			fading := sample()
			scene.Advance(350 * time.Millisecond)
			if _, ok := scene.AccessByRole(ui.RoleProgress, "下载进度"); ok {
				t.Fatal("finished progress never left the card")
			}
			if mode == "active filter" {
				if !scene.HasText("当前没有正在下载的任务") {
					t.Fatal("finished card stayed in the active filter")
				}
				return
			}
			end := sample()
			if fading <= min(full, end) || fading >= max(full, end) {
				t.Errorf("full bar must fade before removal: %d -> %d -> %d", full, fading, end)
			}
			action, _ := scene.AccessByRole(ui.RoleButton, "查看文件详情")
			if _, badge := scene.AccessByRole(ui.RoleStatus, "已完成"); !badge && !strings.Contains(action.Description, "已完成") {
				t.Fatal("missing completed card action or detail status")
			}
		})
	}
}

func TestCompletionStartsFromTheDisplayedFraction(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "photo", Status: "active", Total: 100, Progress: .2}}
	scene := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	task := &v.Snapshot.Tasks[0]
	task.Progress = .8
	scene.Frame()
	scene.Advance(45 * time.Millisecond)
	before, _ := scene.AccessByRole(ui.RoleProgress, "下载进度")
	task.Status, task.Progress = "complete", 1
	scene.Frame()
	after, ok := scene.AccessByRole(ui.RoleProgress, "下载进度")
	if !ok || before.Now <= .2 || before.Now >= .8 || math.Abs(before.Now-after.Now) > .001 {
		t.Fatalf("interrupted progress jumped at completion: %.3f -> %.3f", before.Now, after.Now)
	}
}

func TestCompletionSkipsUnseenProgress(t *testing.T) {
	for _, mode := range []string{"restored", "hidden", "reduced motion", "unknown length"} {
		t.Run(mode, func(t *testing.T) {
			v := presentation.New(nil, presentation.Actions{})
			task := application.TaskDTO{ID: "a", Name: "photo", Status: "active", Total: 100, Progress: .6}
			if mode == "restored" {
				task.Status, task.Progress = "complete", 1
			}
			if mode == "unknown length" {
				task.Total, task.Progress = 0, 0
			}
			v.Snapshot.Tasks = []application.TaskDTO{task}
			scene := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: mode == "reduced motion", ReduceTransparency: true}})
			if mode == "hidden" {
				v.Query = "other"
				scene.Frame()
			}
			v.Snapshot.Tasks[0].Status, v.Snapshot.Tasks[0].Progress = "complete", 1
			v.Query = ""
			scene.Frame()
			bar, visible := scene.AccessByRole(ui.RoleProgress, "下载进度")
			if mode == "unknown length" {
				if !visible || bar.Now != 1 || !scene.HasText("100%") {
					t.Fatal("a completed indeterminate download must show its confirmed endpoint")
				}
			} else if visible {
				t.Fatal("unexpected completion animation")
			}
		})
	}
}
