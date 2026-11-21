package transfer_test

import (
	"bytes"
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

func TestRenderAdvancedEditorsChangeInPlace(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	source, _ := tt.AccessByRole(ui.RoleTextField, "下载链接")
	click("高级选项")
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	entering, _ := tt.AccessByRole(ui.RoleTextField, "单文件另存为")
	tt.Advance(time.Second)
	field, _ := tt.AccessByRole(ui.RoleTextField, "单文件另存为")
	if entering.Bounds.X != field.Bounds.X {
		t.Errorf("opening the inspector moved the field sideways: %.1f -> %.1f", entering.Bounds.X, field.Bounds.X)
	}
	// Sample the label column. Its ink must pass through intermediate
	// intensities even though the labels no longer move laterally.
	crop := func() []byte {
		im := tt.Image()
		var pixels []byte
		for y := int(field.Bounds.Y); y < int(field.Bounds.Y+field.Bounds.H); y++ {
			start := im.PixOffset(int(source.Bounds.X), y)
			end := im.PixOffset(int(field.Bounds.X)-4, y)
			pixels = append(pixels, im.Pix[start:end]...)
		}
		return pixels
	}
	before := crop()
	click("请求头")
	tt.Advance(65 * time.Millisecond)
	tt.Frame()
	incoming, ok := tt.AccessByRole(ui.RoleTextField, "Cookie")
	if !ok || incoming.Bounds.X != field.Bounds.X {
		t.Errorf("tab switch moved the field sideways: %+v, want column %.1f", incoming.Bounds, field.Bounds.X)
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "单文件另存为"); ok {
		t.Fatal("outgoing editor still accepts input")
	}
	mid := crop()
	tt.Advance(time.Second)
	tt.Frame()
	end := crop()
	if bytes.Equal(before, mid) || bytes.Equal(mid, end) || bytes.Equal(before, end) {
		t.Fatal("editor skipped the rendered intermediate state")
	}
	click("文件与连接")
	tt.Advance(time.Second)
	click("代理")
	for range 12 {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if _, visible := tt.Find("Cookie"); visible {
			t.Fatal("a direct file-to-proxy switch rendered the unselected headers")
		}
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
