package transfer_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestWorkspaceCardsKeepTheirSizeAndAlignmentOnResize(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{
		{ID: "c", Name: "third.zip", Status: "error"},
		{ID: "b", Name: "second.zip", Status: "paused"},
		{ID: "a", Name: "first.zip", Status: "active", CanPause: true},
	}
	const height = 800
	tt := ui.NewTesterWith(v.Render, 800, height, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	tt.SetTitleBar(ui.TitleBar{Bottom: 33})
	for _, size := range []struct{ width, columns int }{{800, 1}, {1000, 2}, {1200, 2}, {1440, 3}, {830, 1}} {
		tt.SetSize(size.width, height)
		tt.Frame()
		get := func(label string) ui.Rect {
			t.Helper()
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%d: missing %s", size.width, label)
			}
			return r
		}
		card := get("first.zip")
		nav := get("全部任务")
		if card.W != 336 || card.H != 156 || card.Y != nav.Y {
			t.Errorf("%d: card %v, navigation starts at %.1f", size.width, card, nav.Y)
		}
		for i, name := range []string{"second.zip", "third.zip"} {
			next := get(name)
			column, row := (i+1)%size.columns, (i+1)/size.columns
			if next.X != card.X+float32(column)*348 || next.Y != card.Y+float32(row)*168 || next.W != card.W || next.H != card.H {
				t.Errorf("%d: %s at %v, first %v; want column %d row %d", size.width, name, next, card, column, row)
			}
		}
		bar := get("下载工具栏")
		settings := get("设置")
		if bar.X != card.X || bar.W != card.W || bar.Y+bar.H != settings.Y+settings.H || bar.Y+bar.H != height-23 {
			t.Errorf("%d: bottom toolbar %v, card %v, settings %v", size.width, bar, card, settings)
		}
	}
}

func TestWorkspaceDownloadSheetSharesTheSearchBarAnchor(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	bar, found := tt.AccessByRole(ui.RoleToolbar, "下载工具栏")
	if !found {
		t.Fatal("missing download toolbar")
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	sheet, found := tt.AccessByRole(ui.RoleDialog, "")
	if !found || sheet.Bounds.X != bar.Bounds.X || sheet.Bounds.Y+sheet.Bounds.H != bar.Bounds.Y+bar.Bounds.H {
		t.Fatalf("sheet %v does not open from the leading and bottom edges of the search bar %v", sheet.Bounds, bar.Bounds)
	}
}

func TestWorkspaceFirstPaintAndPartialRowUseTheSameColumns(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	for _, name := range []string{"e", "d", "c", "b", "a"} {
		v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: name, Name: name})
	}
	// A new grid has no previous measurement. Also check the bottom row,
	// where the empty third place must not shift the second card sideways.
	tt := ui.NewTesterWith(v.Render, 1440, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	top, _ := tt.Find("查看详情 b")
	bottom, _ := tt.Find("查看详情 e")
	if top.X != bottom.X || top.W != bottom.W || top.Y >= bottom.Y {
		t.Errorf("partial row moved its second column: %v -> %v", top, bottom)
	}
	tt = ui.NewTesterWith(v.Render, 800, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	narrow, _ := tt.Find("查看详情 a")
	if narrow.W != top.W {
		t.Errorf("first paint: header width %.1f, want %.1f", narrow.W, top.W)
	}
}

func TestWorkspaceDetailAlignsWithNavigationAndCollapsesInPlace(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one.zip", Status: "active", CanPause: true}}
	tt := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	if err := tt.Click("查看详情 one.zip"); err != nil {
		t.Fatal(err)
	}
	detail, ok := tt.AccessByRole(ui.RoleGroup, "one.zip")
	nav, _ := tt.Find("全部任务")
	if !ok || detail.Bounds.Y != nav.Y {
		t.Fatalf("detail %v, navigation %v", detail.Bounds, nav)
	}
	if _, found := tt.Find("返回任务"); found {
		t.Error("duplicate back button")
	}
	if _, found := tt.Find("收起详情 one.zip"); found {
		t.Error("duplicate close control")
	}
	if _, found := tt.Find("返回任务列表"); !found {
		t.Error("missing return action for active task")
	}
	if err := tt.Click("返回任务列表"); err != nil {
		t.Fatal(err)
	}
	if _, found := tt.Find("查看详情 one.zip"); !found {
		t.Fatal("did not return to card")
	}
}

func TestWorkspaceCategoryContentFadesAcrossFrames(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one.zip", Status: "active", Total: 100, Progress: 1}}
	tt := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	track, ok := tt.Find("下载进度")
	if !ok {
		t.Fatal("missing progress track")
	}
	sample := func() uint8 { return tt.Image().RGBAAt(int(track.X+track.W/2), int(track.Y+track.H/2)).R }
	start := sample()
	if err := tt.Click("已暂停"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(90 * time.Millisecond)
	mid := sample()
	tt.Advance(time.Second)
	end := sample()
	if min(start, end) >= mid || mid >= max(start, end) {
		t.Fatalf("category transition skipped intermediate opacity: %d -> %d -> %d", start, mid, end)
	}
}

func TestWorkspaceInterruptExpansionFromEmptySpace(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one.zip"}}
	tt := ui.NewTester(v.Render, 1020, 680)
	if err := tt.Click("查看详情 one.zip"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(80 * time.Millisecond)
	tt.ClickAt(850, 465)
	if _, ok := tt.Find("查看详情 one.zip"); !ok {
		t.Fatal("clicking the empty workspace did not reverse expansion")
	}
	if err := tt.Click("查看详情 one.zip"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if _, ok := tt.Find("来源"); !ok {
		t.Fatal("collapse could not be reversed")
	}
}

func TestWorkspaceTerminalTracksUseQuietSteelInBothThemes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, status := range []string{"error", "complete"} {
			v := presentation.New(nil, presentation.Actions{})
			progress := 0.5
			if status == "complete" {
				progress = 1
			}
			v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "file.zip", Status: status, Total: 100, Progress: progress}}
			tt := ui.NewTesterWith(v.Render, 800, 600, ui.TesterOptions{Dark: dark, Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true}})
			r, ok := tt.Find("下载进度")
			if status == "complete" {
				if ok {
					t.Error("completed track still visible")
				}
				continue
			}
			if !ok {
				t.Fatal("missing track")
			}
			col := tt.Image().RGBAAt(int(r.X+r.W*.15), int(r.Y+r.H/2))
			if col.B <= col.R {
				t.Errorf("%s dark=%v: unexpected track color %v", status, dark, col)
			}
		}
	}
}

func TestWorkspaceCardBodyOpensDetailAndKeepsActionsIndependent(t *testing.T) {
	revealed := ""
	v := presentation.New(nil, presentation.Actions{Reveal: func(path string) { revealed = path }})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one.zip", Status: "active", Path: "/downloads/one.zip", CanPause: true, Total: 100, Progress: .4}}
	tt := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	if err := tt.Click("在文件夹中显示"); err != nil {
		t.Fatal(err)
	}
	if revealed != "/downloads/one.zip" || tt.HasText("来源") {
		t.Fatal("file action opened details or failed to reveal")
	}
	for _, region := range []string{"background", "progress"} {
		r, _ := tt.Find("one.zip")
		x, y := r.X+r.W-12, r.Y+r.H-14
		if region == "progress" {
			bar, _ := tt.Find("下载进度")
			x, y = bar.X+bar.W/2, bar.Y+bar.H/2
		}
		tt.ClickAt(x, y)
		if !tt.HasText("来源") {
			t.Fatalf("%s did not open details", region)
		}
		if _, ok := tt.Find("移除任务（保留文件）"); ok {
			t.Fatal("detail exposes remove action")
		}
		if err := tt.Click("返回任务列表"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkspaceCardContentAlignsAcrossPreviewAndDetail(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "photo.jpg", Status: "complete", Total: 8388608, URL: "https://example.org/photo.jpg", Path: "/downloads/photo.jpg"}}
	tt := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	get := func(label string) ui.Rect {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal(label)
		}
		return r
	}
	size := get("8.0 MB")
	action := get("查看文件详情")
	if err := tt.Click("查看详情 photo.jpg"); err != nil {
		t.Fatal(err)
	}
	detailSize := get("8.0 MB")
	source := get("https://example.org/photo.jpg")
	path := get("/downloads/photo.jpg")
	back := get("返回任务列表")
	if size.X != detailSize.X || detailSize.X != source.X || path.X != source.X || action.X != back.X || back.W > 104 {
		t.Fatalf("preview size %v action %v; detail size %v source %v path %v action %v", size, action, detailSize, source, path, back)
	}
	card, _ := tt.AccessByRole(ui.RoleGroup, "photo.jpg")
	rowGap := path.Y - (source.Y + source.H)
	actionGap := back.Y - (path.Y + path.H)
	bottom := card.Bounds.Y + card.Bounds.H - (back.Y + back.H)
	if rowGap != 6 || actionGap != 16 || bottom != 16 {
		t.Errorf("detail gaps: between fields %.1f, before actions %.1f, below actions %.1f", rowGap, actionGap, bottom)
	}
}
