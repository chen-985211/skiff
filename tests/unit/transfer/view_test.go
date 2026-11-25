package transfer_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestDownloadViewFiltersAndShowsActualTaskProgress(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot = application.Snapshot{Directory: "/downloads", Tasks: []application.TaskDTO{
		{ID: "a", Name: "active.zip", URL: "https://example.com/active.zip", Status: "active", Progress: .25, Total: 1024, Completed: 256, Speed: 128, CanPause: true},
		{ID: "b", Name: "done.zip", URL: "https://example.com/done.zip", Status: "complete", Progress: 1},
	}}
	v.Filter = "complete"
	tt := interactionScene(v, 800, 560)
	if !tt.HasText("done.zip") || tt.HasText("active.zip") {
		t.Fatalf("filter incorrect: %v", tt.Texts())
	}
	v.Filter = "all"
	v.Query = "active"
	tt = interactionScene(v, 800, 560)
	if !tt.HasText("active.zip") || tt.HasText("done.zip") {
		t.Fatalf("search incorrect: %v", tt.Texts())
	}
}

func TestEngineStartupErrorIsVisibleAndDisablesNewDownload(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.EngineError = application.ErrEngineMissing
	tt := interactionScene(v, 800, 560)
	if !tt.HasText("未安装下载引擎") {
		t.Fatal("engine failure hidden")
	}
	title, ok := tt.Find("下载引擎未就绪")
	reason, _ := tt.Find("未安装下载引擎")
	if !ok || reason.Y < title.Y+title.H {
		t.Fatalf("engine failure should explain the empty state below its title: %v, %v", title, reason)
	}
	_ = tt.Click("新建下载")
	if v.DialogOpen {
		t.Fatal("new download opened before engine was ready")
	}
}

func TestWorkspaceSpeedOnlyAppearsOnVisibleActiveTasks(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot = application.Snapshot{Active: 1, Speed: 2048, Tasks: []application.TaskDTO{
		{ID: "a", Name: "a.zip", Status: "active", Speed: 2048},
		{ID: "b", Name: "b.zip", Status: "complete"},
		{ID: "c", Name: "c.iso", Status: "complete"},
	}}
	v.Filter, v.Query = "complete", "ZIP"
	tt := interactionScene(v, 800, 560)
	for _, label := range []string{"1 个结果", "b.zip"} {
		if !tt.HasText(label) {
			t.Errorf("missing %q in filtered workspace: %v", label, tt.Texts())
		}
	}
	if tt.HasText("2.0 KB/s") {
		t.Error("filtered workspace shows the aggregate speed")
	}
	v.Filter, v.Query = "all", ""
	tt.Frame()
	task, _ := tt.Find("a.zip")
	speed, ok := tt.Find("2.0 KB/s")
	if !ok || speed.X < task.X || speed.X+speed.W > task.X+task.W {
		t.Errorf("expected speed on active card: %v, %v", speed, task)
	}

}

func TestWorkspaceEmptySearchClearsFromSearchField(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Filter, v.Query = "paused", "missing"
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "archive.zip", Status: "paused"}}
	tt := interactionScene(v, 800, 560)
	if !tt.HasText("没有匹配的任务") {
		t.Fatalf("missing search empty state: %v", tt.Texts())
	}
	if _, ok := tt.Find("清除搜索"); ok {
		t.Error("empty state duplicates the search field's clear button")
	}
	if err := tt.Click("清空搜索框"); err != nil {
		t.Fatal(err)
	}
	if v.Query != "" || v.Filter != "paused" || !tt.HasText("archive.zip") {
		t.Fatalf("clearing search did not restore this category: %v", tt.Texts())
	}
}

func TestWorkspaceEmptyCategoriesUseSidebarForAllTasks(t *testing.T) {
	for _, tc := range []struct{ category, message, otherStatus string }{
		{"正在下载", "当前没有正在下载的任务", "complete"},
		{"已暂停", "没有暂停的任务", "complete"},
		{"已完成", "还没有已完成的任务", "paused"},
		{"下载失败", "没有失败的任务", "complete"},
	} {
		t.Run(tc.category, func(t *testing.T) {
			v := presentation.New(nil, presentation.Actions{})
			v.Ready = true
			v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "archive.zip", Status: tc.otherStatus}}
			tt := interactionScene(v, 800, 560)
			if err := tt.Click(tc.category); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(tc.message) {
				t.Fatalf("missing category empty state: %v", tt.Texts())
			}
			if _, ok := tt.Find("查看全部任务"); ok {
				t.Error("empty state duplicates the sidebar navigation")
			}
			if err := tt.Click("全部任务"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText("archive.zip") || v.Filter != "all" {
				t.Fatal("sidebar did not restore all tasks")
			}
		})
	}
}

func TestWorkspaceHasNoDecorativeHeadingsOrSubtitles(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := interactionScene(v, 800, 560)
	for _, copy := range []string{"SKIFF", "任务库", "传输概览", "粘贴 HTTP 或 HTTPS 链接，开始第一次下载。"} {
		if tt.HasText(copy) {
			t.Errorf("unexpected decorative text %q", copy)
		}
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("支持 HTTP 和 HTTPS，每行一个链接。") {
		t.Fatal("form has a subtitle")
	}
}

func TestSearchShortcutTargetsSearchAndDoesNotStealModalFocus(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := interactionScene(v, 800, 560)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("skiff")
	if v.Query != "skiff" {
		t.Fatalf("search did not receive text: %q", v.Query)
	}
	v.OpenNew()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("https://example.org/a")
	if v.URLs != "https://example.org/a" || v.Query != "skiff" {
		t.Fatal("search shortcut intercepted modal input")
	}
}

func TestWorkspaceBatchActionsFollowEligibleSearchResults(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	_, err := s.Add(context.Background(), "https://example.org/visible-a.zip\nhttps://example.org/visible-b.zip\nhttps://example.org/hidden.zip", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { updates <- fn }})
	v.Snapshot, v.Query = s.Snapshot(), "visible"
	tt := interactionScene(v, 800, 560)
	click := func(label string) {
		t.Helper()
		if !tt.HasText(label) {
			t.Fatalf("missing visible action %q", label)
		}
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		select {
		case update := <-updates:
			update()
			tt.Frame()
		case <-time.After(3 * time.Second):
			t.Fatal("bulk action did not finish")
		}
	}
	click("暂停 2 项")
	tasks := v.Snapshot.Tasks
	if tasks[0].Status != "paused" || tasks[1].Status != "paused" || tasks[2].Status != "waiting" {
		t.Fatalf("unexpected bulk pause scope: %+v", tasks)
	}
	v.Query = "visible-a"
	tt.Frame()
	if _, ok := tt.Find("继续 1 项"); ok || tt.HasText("继续 2 项") {
		t.Fatal("single search result exposes a batch action")
	}
	if _, ok := tt.Find("继续下载"); !ok {
		t.Fatal("missing individual resume action")
	}
	v.Query = ""
	tt.Frame()
	if _, ok := tt.Find("暂停 1 项"); ok {
		t.Fatal("single waiting task exposes a bulk pause action")
	}
	click("继续 2 项")
	for _, task := range v.Snapshot.Tasks {
		if task.Status != "waiting" {
			t.Fatalf("unexpected task status after resume: %+v", task)
		}
	}
}

func TestWorkspaceShowsDestinationFolderName(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Snapshot.Directory = "/some/long/path/to/电影素材"
	tt := interactionScene(v, 800, 560)
	if !slices.Contains(tt.Texts(), "电影素材") {
		t.Fatal("destination name is truncated behind its ancestor directories")
	}
}

func TestWorkspaceNavigationAndDockPositions(t *testing.T) {
	for _, width := range []int{800, 1020} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "archive.zip", Status: "paused", CanResume: true}}
		tt := interactionScene(v, width, 600)
		all, _ := tt.AccessByRole(ui.RoleButton, "全部任务")
		paused, _ := tt.AccessByRole(ui.RoleButton, "已暂停")
		search, _ := tt.Find("搜索下载任务")
		add, _ := tt.Find("新建下载")
		file, _ := tt.Find("查看详情 archive.zip")
		if paused.Bounds.Y <= all.Bounds.Y+all.Bounds.H || paused.Bounds.X != all.Bounds.X || file.X < all.Bounds.X+all.Bounds.W {
			t.Errorf("width=%d: filters are not beside the file workspace", width)
		}
		if search.Y < 490 || search.Y >= add.Y+add.H || search.X+search.W > add.X {
			t.Errorf("width=%d: search and new download do not share the bottom bar: %+v %+v", width, search, add)
		}
		v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: "b", Name: "new.zip"})
		tt.Frame()
		withNeighbor, _ := tt.Find("查看详情 archive.zip")
		if file.W != withNeighbor.W {
			t.Errorf("width=%d: task resized when a neighbor appeared: %v -> %v", width, file.W, withNeighbor.W)
		}

	}
}

func TestQuietQueueKeepsActionsAndSuppressesRedundantCopy(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := interactionScene(v, 900, 600)
	for _, redundant := range []string{"添加第一个下载", "0 B/s", "0"} {
		if slices.Contains(tt.Texts(), redundant) {
			t.Errorf("idle queue repeats %q", redundant)
		}
	}
	if _, ok := tt.AccessByRole(ui.RoleStatus, "引擎就绪"); ok {
		t.Error("idle engine indicator should be removed")
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, "新建下载"); !ok {
		t.Error("missing create action")
	}
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "done.zip", Status: "complete", Completed: 1024, Total: 1024, Progress: 1}}
	tt.Frame()
	for _, redundant := range []string{"1.0 KB / 1.0 KB", "100%"} {
		if slices.Contains(tt.Texts(), redundant) {
			t.Errorf("completed card repeats %q", redundant)
		}
	}
	task, _ := tt.AccessByRole(ui.RoleButton, "查看详情 done.zip")
	if strings.Count(task.Description, "已完成") != 1 || !tt.HasText("1.0 KB") {
		t.Errorf("missing accessible state or file size: %+v %v", task, tt.Texts())
	}
}

func TestRenderCategorySelectionTravelsAcrossIntermediateRow(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce, ReduceTransparency: true}})
		row, _ := tt.AccessByRole(ui.RoleButton, "正在下载")
		x, y := int(row.Bounds.X+5), int(row.Bounds.Y+row.Bounds.H/2)
		original := tt.Image().RGBAAt(x, y)
		if err := tt.Click("已暂停"); err != nil {
			t.Fatal(err)
		}
		changed := false
		for range 12 {
			tt.Advance(15 * time.Millisecond)
			if tt.Image().RGBAAt(x, y) != original {
				changed = true
			}
		}
		if changed == reduce {
			t.Errorf("reduced=%v, intermediate row visibly highlighted=%v", reduce, changed)
		}
	}
}

func TestRenderWindowLightMovesAtRestWithoutSidebarSeam(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
		tt.Advance(time.Second)
		sample := func() []byte {
			im := tt.Image()
			var pixels []byte
			for x := 32; x < 890; x += 16 {
				color := im.RGBAAt(x, 438)
				pixels = append(pixels, color.R, color.G, color.B)
			}
			return pixels
		}
		first := sample()
		tt.Advance(2 * time.Second)
		if slices.Equal(first, sample()) != reduce {
			t.Errorf("reduced=%v: unexpected idle light movement", reduce)
		}
		im := tt.Image()
		left, right := im.RGBAAt(173, 438), im.RGBAAt(174, 438)
		if d := int(left.R) - int(right.R); d > 4 || d < -4 {
			t.Errorf("sidebar seam %v -> %v", left, right)
		}
	}
}

func TestSettingsReplacesEngineLightAndReturnsToQueue(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Filter = "paused"
	v.Snapshot.Directory = t.TempDir()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("默认保存目录") || !tt.HasText("背景流光") || !tt.HasText(v.Snapshot.Directory) {
		t.Fatal("settings not displayed")
	}
	if err := tt.Click("已暂停"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("默认保存目录") || !tt.HasText("没有暂停的任务") {
		t.Fatal("did not return to queue")
	}
}

func TestSidebarOpensTheDirectoryItShows(t *testing.T) {
	dir := t.TempDir()
	var opened string
	v := presentation.New(nil, presentation.Actions{OpenDirectory: func(path string, done func(error)) { opened = path; done(nil) }})
	v.Snapshot.Directory = dir
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("打开下载目录"); err != nil {
		t.Fatal(err)
	}
	if opened != dir {
		t.Fatalf("opened %q, want %q", opened, dir)
	}
}

func TestSettingsControlsSaveAndSupplyNextDownload(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	dir := t.TempDir()
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { done <- fn }, PickDirectory: func(_ string, choose func(string, error)) { choose(dir, nil) }})
	v.Snapshot = s.Snapshot()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	wait := func() {
		t.Helper()
		select {
		case fn := <-done:
			fn()
			tt.Frame()
		case <-time.After(2 * time.Second):
			t.Fatal("no settings update")
		}
	}
	v.OpenNew()
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	click("设置")
	click("更改默认保存目录")
	wait()
	click("背景流光")
	wait()
	if !v.Snapshot.Preferences.StaticBackground || v.Snapshot.Directory != dir {
		t.Fatalf("settings not saved: %+v", v.Snapshot)
	}
	v.OpenNew()
	if v.Directory != dir {
		t.Fatalf("new download uses %q instead of %q", v.Directory, dir)
	}
}

func TestSettingsChangeQueueAndPrioritizeWaitingCard(t *testing.T) {
	f := &fakeEngine{}
	s := service(t, f, &fakeRepo{})
	dir := s.Snapshot().Directory
	_, _ = s.Add(t.Context(), "https://example.org/queued", dir)
	dispatch := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { dispatch <- fn }})
	v.Snapshot = s.Snapshot()
	tt := interactionScene(v, 800, 560)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	apply := func() {
		t.Helper()
		select {
		case fn := <-dispatch:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("missing UI update")
		}
		tt.Frame()
	}
	click("设置")
	click("同时下载 20 项")
	apply()
	if s.Snapshot().Preferences.Concurrency != 20 {
		t.Fatal("setting did not reach engine")
	}
	click("全部任务")
	click("优先下载")
	apply()
	if len(f.commands) != 1 || f.commands[0] != application.Prioritize {
		t.Fatal("priority action lost")
	}
}
