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

func TestEmptyDownloadViewAndNewDownloadValidation(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Directory = "/downloads"
	tt := interactionScene(v, 1000, 650)
	if !tt.HasText("还没有下载任务") {
		t.Fatalf("empty state missing: %v", tt.Texts())
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	if !v.DialogOpen {
		t.Fatal("new download did not open")
	}
	if err := tt.Click("开始下载"); err != nil {
		t.Fatal(err)
	}
	if v.FormError == "" || !v.DialogOpen {
		t.Fatal("empty download must be rejected without closing form")
	}
}
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
	v.EngineError = errors.New("没有找到 aria2c")
	tt := interactionScene(v, 800, 560)
	if !tt.HasText("没有找到 aria2c") {
		t.Fatal("engine failure hidden")
	}
	title, ok := tt.Find("下载引擎未就绪")
	reason, _ := tt.Find("没有找到 aria2c")
	if !ok || reason.Y < title.Y+title.H {
		t.Fatalf("engine failure should explain the empty state below its title: %v, %v", title, reason)
	}
	_ = tt.Click("新建下载")
	if v.DialogOpen {
		t.Fatal("new download opened before engine was ready")
	}
}

func TestDownloadFormFitsMinimumWindowInBothThemes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.Directory = "/downloads"
		v.DialogOpen = true
		tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Dark: dark, Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true}})
		for _, name := range []string{"下载链接", "保存目录", "开始下载"} {
			rect, ok := tt.Find(name)
			if !ok || rect.X < 0 || rect.Y < 0 || rect.X+rect.W > 800 || rect.Y+rect.H > 560 {
				t.Errorf("dark=%v: %s out of bounds: %+v", dark, name, rect)
			}
		}
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

func TestNewDownloadKeepsCurrentFormAndCountsNonblankLines(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	v.Directory = "/selected"
	v.URLs = "https://example.org/a\n \nhttps://example.org/b\n"
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	if v.Directory != "/selected" || !tt.HasText("已输入 2 行") {
		t.Fatalf("form lost its directory or batch count: %q, %v", v.Directory, tt.Texts())
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

func TestReopenedDraftKeepsDirectoryWithItsLinks(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Directory = t.TempDir()
	v.OpenNew()
	dir := t.TempDir()
	v.Directory, v.URLs = dir, "https://example.org/draft.zip"
	tt := interactionScene(v, 800, 560)
	tt.Key(0, ui.KeyEscape)
	if v.DialogOpen {
		t.Fatal("escape did not dismiss form")
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	if v.Directory != dir || v.URLs != "https://example.org/draft.zip" {
		t.Fatal("draft reopened with the wrong destination or links")
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

func TestOpeningDownloadPreservesCurrentCategory(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("正在下载"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	if !v.DialogOpen || v.Filter != "active" {
		t.Fatalf("unexpected form or category: %v, %s", v.DialogOpen, v.Filter)
	}
}

func TestEditingDownloadLinksClearsStaleValidation(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.DialogOpen = true, true
	v.Directory = t.TempDir()
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("开始下载"); err != nil {
		t.Fatal(err)
	}
	if v.FormError == "" {
		t.Fatal("missing empty-link error")
	}
	field, ok := tt.AccessByRole(ui.RoleTextField, "下载链接")
	if !ok {
		t.Fatal("missing link input")
	}
	r := field.Bounds
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Type("https://example.org/archive.zip")
	if v.FormError != "" || v.URLs == "" {
		t.Fatalf("edited links still show a stale error: %s (%q)", v.FormError, v.URLs)
	}
}

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

func TestSubmittingDialogKeepsResultVisibleUntilSubmissionReturns(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.DialogOpen, v.Busy = true, true, true
	tt := interactionScene(v, 800, 560)
	tt.Key(0, ui.KeyEscape)
	if !v.DialogOpen {
		t.Fatal("escape discarded the dialog while its result was pending")
	}
	tt.ClickAt(8, 8)
	if !v.DialogOpen {
		t.Fatal("backdrop dismissed the pending result")
	}
	v.Busy = false
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if v.DialogOpen {
		t.Fatal("escape should dismiss the editable draft")
	}
}

func TestWorkspacePartialCreationExplainsFailureInForm(t *testing.T) {
	s := service(t, &fakeEngine{addErr: errors.New("引擎暂时不可用")}, &fakeRepo{})
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { done <- fn }})
	v.OpenNew()
	v.URLs, v.Directory = "https://example.org/archive.zip", t.TempDir()
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("开始下载"); err != nil {
		t.Fatal(err)
	}
	select {
	case apply := <-done:
		apply()
	case <-time.After(3 * time.Second):
		t.Fatal("creation did not finish")
	}
	tt.Frame()
	message := "已创建 1 个任务，部分操作失败：引擎暂时不可用"
	if !v.DialogOpen || !tt.HasText(message) || v.URLs != "" || len(v.Snapshot.Tasks) != 1 {
		t.Fatalf("partial creation left the wrong form state: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if tt.HasText(message) || !tt.HasText("引擎暂时不可用") {
		t.Fatal("dismissing the form should leave the affected card's own error visible")
	}
}

func TestCardFailureActionsFitMinimumWindow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.Snapshot.Tasks = []application.TaskDTO{{ID: "f", Name: strings.Repeat("很长的文件名", 20) + ".zip", Status: "error", Error: "远端服务器暂时无法访问", CanRestart: true, CanRetry: true}}
		tt := ui.NewTesterWith(v.Render, 800, 528, ui.TesterOptions{Dark: dark, Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true}})
		for _, label := range []string{"查看详情 " + v.Snapshot.Tasks[0].Name, "远端服务器暂时无法访问", "从头下载", "移除任务（保留文件）"} {
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
	f.addErr = errors.New("引擎暂时不可用")
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
		if label == f.addErr.Error() {
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
	if !tt.HasText("保存任务失败：disk full") {
		t.Fatal("removing the card hid the persistence failure")
	}
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("保存任务失败：disk full") {
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

func TestSheetExpandsFromTheDockWhenOpened(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 1000, 640, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce, ReduceTransparency: true}})
		if err := tt.Click("新建下载"); err != nil {
			t.Fatal(err)
		}
		start, _ := tt.AccessByRole(ui.RoleDialog, "")
		tt.Advance(110 * time.Millisecond)
		middle, _ := tt.AccessByRole(ui.RoleDialog, "")
		tt.Advance(time.Second)
		end, _ := tt.AccessByRole(ui.RoleDialog, "")
		if !reduce && !(start.Bounds.W < middle.Bounds.W && middle.Bounds.W < end.Bounds.W) {
			t.Errorf("sheet did not expand: %.1f, %.1f, %.1f", start.Bounds.W, middle.Bounds.W, end.Bounds.W)
		}
		if reduce && start.Bounds != end.Bounds {
			t.Errorf("reduced-motion sheet moved: %v -> %v", start, end)
		}
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
