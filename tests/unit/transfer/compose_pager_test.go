package transfer_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

func TestAdvancedEditorReplacesLinksAndPreservesTheDraft(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.URLs = true, "https://example.org/archive.zip"
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	footer, _ := tt.Find("开始下载")
	if err := tt.Click("高级选项"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, found := tt.AccessByRole(ui.RoleTextField, "下载链接"); found {
		t.Fatal("link editor competes with the advanced editor")
	}
	name, ok := tt.AccessByRole(ui.RoleTextField, "单文件另存为")
	sheet, _ := tt.AccessByRole(ui.RoleDialog, "")
	if !ok || name.Bounds.Y-sheet.Bounds.Y > 115 {
		t.Fatalf("first setting starts too far below the header: %v in %v", name.Bounds, sheet.Bounds)
	}
	if err := tt.Click("单文件另存为"); err != nil {
		t.Fatal(err)
	}
	tt.Type("books.zip")
	if err := tt.Click("返回链接"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	link, ok := tt.AccessByRole(ui.RoleTextField, "下载链接")
	if !ok || link.Value != v.URLs {
		t.Fatal("returning lost the link")
	}
	if end, _ := tt.Find("开始下载"); end != footer {
		t.Fatal("submit moved", footer, end)
	}
	if err := tt.Click("高级选项"); err != nil {
		t.Fatal(err)
	}
	if n, ok := tt.AccessByRole(ui.RoleTextField, "单文件另存为"); !ok || n.Value != "books.zip" {
		t.Fatal("returning lost the advanced draft")
	}
}

func TestRenderAdvancedPageTravelsIntoPlace(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	if err := tt.Click("高级选项"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	field, _ := tt.AccessByRole(ui.RoleTextField, "单文件另存为")
	rail, _ := tt.Find("高级下载选项")
	if err := tt.Click("请求头"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(85 * time.Millisecond)
	tt.Frame()
	incoming, ok := tt.AccessByRole(ui.RoleTextField, "Cookie")
	if !ok || incoming.Bounds.X <= field.Bounds.X+8 || incoming.Bounds.X >= field.Bounds.X+rail.W {
		t.Fatalf("incoming editor snapped to its endpoint: %v, expected motion relative to %v", incoming.Bounds, field.Bounds)
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "单文件另存为"); ok {
		t.Fatal("leaving field still takes input")
	}
	tt.Advance(time.Second)
	settled, _ := tt.AccessByRole(ui.RoleTextField, "Cookie")
	if settled.Bounds.X != field.Bounds.X {
		t.Fatal("editors do not settle on the same column")
	}
}

func TestEmptyAdvancedSubmitReturnsToEditableLink(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	for _, label := range []string{"高级选项", "开始下载"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	link, ok := tt.AccessByRole(ui.RoleTextField, "下载链接")
	if !ok || !link.Focused || !tt.HasText("请先输入下载链接") {
		t.Fatalf("missing link error at focused input: %+v", link)
	}
}
