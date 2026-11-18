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
	tt := ui.NewTester(v.Render, 1000, 650)
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
	tt := ui.NewTester(v.Render, 800, 560)
	if !tt.HasText("done.zip") || tt.HasText("active.zip") {
		t.Fatalf("filter incorrect: %v", tt.Texts())
	}
	v.Filter = "all"
	v.Query = "active"
	tt = ui.NewTester(v.Render, 800, 560)
	if !tt.HasText("active.zip") || tt.HasText("done.zip") {
		t.Fatalf("search incorrect: %v", tt.Texts())
	}
}
func TestEngineStartupErrorIsVisibleAndDisablesNewDownload(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Message = "没有找到 aria2c"
	tt := ui.NewTester(v.Render, 800, 560)
	if !tt.HasText("没有找到 aria2c") {
		t.Fatal("engine failure hidden")
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
		tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Dark: dark})
		for _, name := range []string{"下载链接", "保存目录", "开始下载", "取消"} {
			rect, ok := tt.Find(name)
			if !ok || rect.X < 0 || rect.Y < 0 || rect.X+rect.W > 800 || rect.Y+rect.H > 560 {
				t.Errorf("dark=%v: %s out of bounds: %+v", dark, name, rect)
			}
		}
	}
}

func TestFilteredQueueShowsResultCountAndKeepsOverallSpeed(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot = application.Snapshot{Active: 1, Speed: 2048, Tasks: []application.TaskDTO{
		{ID: "a", Name: "a.zip", Status: "active"},
		{ID: "b", Name: "b.zip", Status: "complete"},
		{ID: "c", Name: "c.iso", Status: "complete"},
	}}
	v.Filter, v.Query = "complete", "ZIP"
	tt := ui.NewTester(v.Render, 800, 560)
	for _, label := range []string{"2.0 KB/s", "1 个结果", "b.zip"} {
		if !tt.HasText(label) {
			t.Errorf("missing %q in filtered workspace: %v", label, tt.Texts())
		}
	}
}

func TestSearchEmptyStateClearsOnlyQuery(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Filter, v.Query = "paused", "missing"
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "archive.zip", Status: "paused"}}
	tt := ui.NewTester(v.Render, 800, 560)
	if !tt.HasText("没有匹配的任务") {
		t.Fatalf("missing search empty state: %v", tt.Texts())
	}
	if err := tt.Click("清除搜索"); err != nil {
		t.Fatal(err)
	}
	if v.Filter != "paused" || !tt.HasText("archive.zip") {
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
	tt := ui.NewTester(v.Render, 800, 560)
	if v.Directory != "/selected" || !tt.HasText("已输入 2 行") {
		t.Fatalf("form lost its directory or batch count: %q, %v", v.Directory, tt.Texts())
	}
}

func TestEmptyCategoryReturnsToAllTasks(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.Filter = true, "error"
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "ok", Name: "saved.zip", Status: "complete"}}
	tt := ui.NewTester(v.Render, 800, 560)
	if !tt.HasText("没有失败的任务") {
		t.Fatalf("missing category empty state: %v", tt.Texts())
	}
	if err := tt.Click("查看全部任务"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("saved.zip") || v.Filter != "all" {
		t.Fatal("all tasks were not restored")
	}
}

func TestWorkspaceHasNoDecorativeHeadingsOrSubtitles(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTester(v.Render, 800, 560)
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
	tt := ui.NewTester(v.Render, 800, 560)
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
	tt := ui.NewTester(v.Render, 800, 560)
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

func TestBulkPauseAffectsOnlyEligibleSearchResults(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	_, err := s.Add(context.Background(), "https://example.org/visible.zip\nhttps://example.org/hidden.zip", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { updates <- fn }})
	v.Snapshot, v.Query = s.Snapshot(), "visible"
	tt := ui.NewTester(v.Render, 800, 560)
	if err := tt.Click("暂停当前结果"); err != nil {
		t.Fatal(err)
	}
	select {
	case update := <-updates:
		update()
	case <-time.After(3 * time.Second):
		t.Fatal("bulk action did not finish")
	}
	if v.Snapshot.Tasks[0].Status != "paused" || v.Snapshot.Tasks[1].Status != "waiting" {
		t.Fatalf("unexpected bulk scope: %+v", v.Snapshot.Tasks)
	}
	tt.Frame()
	if _, ok := tt.Find("暂停当前结果"); ok {
		t.Fatal("hidden task still enables pause")
	}
}
func TestWorkspaceShowsDestinationFolderName(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Snapshot.Directory = "/some/long/path/to/电影素材"
	tt := ui.NewTester(v.Render, 800, 560)
	if !slices.Contains(tt.Texts(), "电影素材") {
		t.Fatal("destination name is truncated behind its ancestor directories")
	}
}

func TestOpeningDownloadPreservesCurrentCategory(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTester(v.Render, 800, 560)
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
	tt := ui.NewTester(v.Render, 800, 560)
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
	tt := ui.NewTester(v.Render, 800, 560)
	if err := tt.Click("查看详情 manual.zip"); err != nil {
		t.Fatal(err)
	}
	v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: "b", Name: "other.zip", Status: "waiting"})
	tt.Frame()
	if !tt.HasText(task.URL) || !tt.HasText(dir) {
		t.Fatal("source and destination missing after insertion")
	}
	if err := tt.Click("收起详情 manual.zip"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText(task.URL) {
		t.Fatal("detail did not collapse")
	}
}

func TestSubmittingDialogKeepsResultVisibleUntilSubmissionReturns(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.DialogOpen, v.Busy = true, true, true
	tt := ui.NewTester(v.Render, 800, 560)
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

func TestCardFailureActionsFitMinimumWindow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.Snapshot.Tasks = []application.TaskDTO{{ID: "f", Name: strings.Repeat("很长的文件名", 20) + ".zip", Status: "error", Error: "远端服务器暂时无法访问", CanRestart: true, CanRetry: true}}
		tt := ui.NewTesterWith(v.Render, 800, 528, ui.TesterOptions{Dark: dark, Preferences: ui.Preferences{ReduceMotion: true}})
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
	tt := ui.NewTesterWith(v.Render, 1000, 640, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
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
	tt := ui.NewTester(v.Render, 800, 560)
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

func TestFailedCardMakesRecoveryVisible(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "failed", Name: "__down", Status: "error", Error: "下载引擎 HTTP 状态 400", CanRetry: true, CanRestart: true}}
	tt := ui.NewTester(v.Render, 800, 528)
	action, ok := tt.Find("重试下载")
	if !ok || action.W < 72 || !slices.Contains(tt.Texts(), "重试下载") {
		t.Fatal("the primary recovery action is only an icon")
	}
}

func TestSheetExpandsFromTheDockWhenOpened(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 1000, 640, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
		if err := tt.Click("新建下载"); err != nil {
			t.Fatal(err)
		}
		start, _ := tt.Find("开始下载")
		tt.Advance(110 * time.Millisecond)
		middle, _ := tt.Find("开始下载")
		tt.Advance(time.Second)
		end, _ := tt.Find("开始下载")
		if !reduce && !(start.W < middle.W && middle.W < end.W) {
			t.Errorf("sheet did not expand: %.1f, %.1f, %.1f", start.W, middle.W, end.W)
		}
		if reduce && start != end {
			t.Errorf("reduced-motion sheet moved: %v -> %v", start, end)
		}
	}
}

func TestWorkspaceNavigationAndDockPositions(t *testing.T) {
	for _, width := range []int{800, 1020} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "archive.zip", Status: "paused", CanResume: true}}
		tt := ui.NewTesterWith(v.Render, width, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
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
		if width == 1020 && file.X+file.W < float32(width)-90 {
			t.Errorf("single file leaves half the workspace empty: %+v", file)
		}
	}
}

func TestQuietQueueKeepsActionsAndSuppressesRedundantCopy(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTester(v.Render, 900, 600)
	for _, redundant := range []string{"添加第一个下载", "0 B/s", "0"} {
		if slices.Contains(tt.Texts(), redundant) {
			t.Errorf("idle queue repeats %q", redundant)
		}
	}
	if state, ok := tt.AccessByRole(ui.RoleStatus, "引擎就绪"); !ok || state.Bounds.W > 16 {
		t.Error("engine state inaccessible")
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

func TestTransferFlowTracksActivityAndMotionPreference(t *testing.T) {
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

func TestCategorySelectionTravelsAcrossIntermediateRow(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
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
