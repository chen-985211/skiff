package transfer_test

import (
	"testing"

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
		if card.W != 376 || card.H != 188 || card.Y != nav.Y {
			t.Errorf("%d: card %v, navigation starts at %.1f", size.width, card, nav.Y)
		}
		for i, name := range []string{"second.zip", "third.zip"} {
			next := get(name)
			column, row := (i+1)%size.columns, (i+1)/size.columns
			if next.X != card.X+float32(column)*388 || next.Y != card.Y+float32(row)*200 || next.W != card.W || next.H != card.H {
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
