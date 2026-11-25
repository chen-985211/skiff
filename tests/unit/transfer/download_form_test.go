package transfer_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
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
	if v.FormError == nil || !v.DialogOpen {
		t.Fatal("empty download must be rejected without closing form")
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
	if v.FormError == nil {
		t.Fatal("missing empty-link error")
	}
	field, ok := tt.AccessByRole(ui.RoleTextField, "下载链接")
	if !ok {
		t.Fatal("missing link input")
	}
	r := field.Bounds
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Type("https://example.org/archive.zip")
	if v.FormError != nil || v.URLs == "" {
		t.Fatalf("edited links still show a stale error: %v (%q)", v.FormError, v.URLs)
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
	s := service(t, &fakeEngine{addErr: application.ErrEngineConnection}, &fakeRepo{})
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
	message := "已创建 1 个任务，部分操作失败：无法连接下载引擎"
	if !v.DialogOpen || !tt.HasText(message) || v.URLs != "" || len(v.Snapshot.Tasks) != 1 {
		t.Fatalf("partial creation left the wrong form state: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if tt.HasText(message) || !tt.HasText("无法连接下载引擎") {
		t.Fatal("dismissing the form should leave the affected card's own error visible")
	}
}

type rejectFailedDownloadsRepository struct{ fakeRepo }

func (r *rejectFailedDownloadsRepository) Save(ctx context.Context, state application.State) error {
	for _, task := range state.Tasks {
		if task.Status == domain.Failed {
			return errors.New("private repository detail")
		}
	}
	return r.fakeRepo.Save(ctx, state)
}

func TestDownloadFormLocalizesEachDistinctBatchFailure(t *testing.T) {
	next := 0
	api := application.New(&fakeEngine{addErr: application.ErrEngineConnection}, &rejectFailedDownloadsRepository{}, t.TempDir(), func() string {
		next++
		return fmt.Sprintf("%016x", next)
	})
	if err := api.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	done := make(chan func(), 1)
	view := presentation.New(api, presentation.Actions{Dispatch: func(fn func()) { done <- fn }})
	view.OpenNew()
	view.URLs, view.Directory = "https://example.org/a\nhttps://example.org/b", t.TempDir()
	scene := interactionScene(view, 800, 560)
	chooseSource(t, scene, "开始下载")
	select {
	case apply := <-done:
		apply()
	case <-time.After(3 * time.Second):
		t.Fatal("batch creation did not finish")
	}
	scene.Frame()
	if !scene.HasText("已创建 2 个任务，部分操作失败：无法连接下载引擎\n保存任务失败") {
		t.Errorf("Chinese form did not show both distinct failures: %v", scene.Texts())
	}
	scene.SetLocale("en")
	if !scene.HasText("Created 2 tasks. Some operations failed: Could not connect to the download engine\nCould not save the download queue") {
		t.Errorf("English form did not show both distinct failures: %v", scene.Texts())
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

func TestBatchFormRetainsOnlyRowsThatNeedAttention(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	dispatch := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { dispatch <- fn }})
	v.Snapshot = s.Snapshot()
	v.OpenNew()
	v.URLs = "https://example.org/a\ninvalid\nhttps://example.org/a"
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("开始下载"); err != nil {
		t.Fatal(err)
	}
	select {
	case apply := <-dispatch:
		apply()
	case <-time.After(3 * time.Second):
		t.Fatal("no batch result")
	}
	tt.Frame()
	if !v.DialogOpen || v.URLs != "invalid" || len(v.Snapshot.Tasks) != 1 || !tt.HasText("新增 1 · 重复 1 · 无效 1") {
		t.Fatalf("draft %q; %v", v.URLs, tt.Texts())
	}
}

func TestAdvancedConnectionLimitAndLiveCount(t *testing.T) {
	f := &fakeEngine{}
	s := service(t, f, &fakeRepo{})
	dispatch := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { dispatch <- fn }})
	v.Snapshot = s.Snapshot()
	v.OpenNew()
	v.URLs = "https://example.org/file.bin"
	tt := interactionScene(v, 800, 560)
	if _, ok := tt.AccessByRole(ui.RoleTextField, "单文件连接上限"); ok {
		t.Fatal("advanced field opened by default")
	}
	if err := tt.Click("高级选项"); err != nil {
		t.Fatal(err)
	}
	focusLimit := func() {
		field, ok := tt.AccessByRole(ui.RoleTextField, "单文件连接上限")
		if !ok {
			t.Fatal("missing limit field")
		}
		r := field.Bounds
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	}
	focusLimit()
	tt.Type("17")
	_ = tt.Click("开始下载")
	if !v.DialogOpen || v.FormError == nil {
		t.Fatal("invalid cap accepted")
	}
	focusLimit()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("2")
	_ = tt.Click("开始下载")
	select {
	case apply := <-dispatch:
		apply()
	case <-time.After(4 * time.Second):
		t.Fatal("submit failed")
	}
	if v.DialogOpen || len(f.enqueued) != 1 || f.enqueued[0].ConnectionLimit != 2 {
		t.Fatalf("bad spec %+v", f.enqueued)
	}
	id := v.Snapshot.Tasks[0].ID
	f.observed = []application.Observation{{ID: id, Status: "active", Connections: 2, Total: 24 << 20}}
	if err := s.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	v.Snapshot = s.Snapshot()
	tt.Frame()
	if err := tt.Click("查看详情 file.bin"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("2 / 2") {
		t.Fatalf("missing live count: %v", tt.Texts())
	}
}

func TestOpenDownloadValidationUpdatesWhenLocaleChanges(t *testing.T) {
	view := presentation.New(nil, presentation.Actions{})
	view.Ready = true
	view.Snapshot.Directory = t.TempDir()
	scene := interactionScene(view, 800, 560)
	chooseSource(t, scene, "新建下载")
	chooseSource(t, scene, "开始下载")
	if !scene.HasText("请先输入下载链接") {
		t.Fatal("missing initial validation message")
	}
	scene.SetLocale("en")
	if !scene.HasText("Enter a download link") || scene.HasText("请先输入下载链接") {
		t.Fatalf("validation kept the previous locale: %v", scene.Texts())
	}
}

func TestBatchDownloadNoticeUpdatesWhenLocaleChanges(t *testing.T) {
	svc := service(t, &fakeEngine{}, &fakeRepo{})
	updates := make(chan func(), 1)
	view := presentation.New(svc, presentation.Actions{
		Context:  t.Context(),
		Dispatch: func(fn func()) { updates <- fn },
	})
	view.Snapshot = svc.Snapshot()
	view.OpenNew()
	view.URLs = "https://example.test/one.zip\nhttps://example.test/two.zip"
	scene := interactionScene(view, 800, 560)
	chooseSource(t, scene, "开始下载")
	select {
	case update := <-updates:
		update()
		scene.Frame()
	case <-time.After(time.Second):
		t.Fatal("batch result did not arrive")
	}
	const chinese = "新增 2 · 重复 0 · 无效 0"
	if !scene.HasText(chinese) {
		t.Fatalf("missing initial batch notice: %v", scene.Texts())
	}
	scene.SetLocale("en")
	if !scene.HasText("Added 2 · Duplicates 0 · Invalid 0") || scene.HasText(chinese) {
		t.Fatalf("batch notice kept the previous locale: %v", scene.Texts())
	}
}
