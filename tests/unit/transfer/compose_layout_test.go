package transfer_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation"
)

func TestComposeLinkFieldGrowsWithinTheAnchoredSheet(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	links := func() ui.Rect {
		t.Helper()
		n, ok := tt.AccessByRole(ui.RoleTextField, "下载链接")
		if !ok {
			t.Fatal("missing link editor")
		}
		return n.Bounds
	}
	empty := links()
	submit, _ := tt.Find("开始下载")
	if empty.H < 4*20+24 {
		t.Errorf("empty editor is %.1f pt tall; want room for four lines and padding", empty.H)
	}
	paste := func(n int) {
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(strings.TrimSuffix(strings.Repeat("https://example.org/file.zip\n", n), "\n"))
	}
	paste(5)
	batch := links()
	count, ok := tt.Find("已输入 5 行")
	if batch.H <= empty.H || !ok || count.Y+count.H > batch.Y {
		t.Errorf("batch count should sit above a growing editor: count=%v empty=%v batch=%v", count, empty, batch)
	}
	paste(8)
	capped := links()
	paste(12)
	if many := links(); many.H != capped.H || many.H <= batch.H || many.Y < 0 {
		t.Errorf("large batches should scroll within the editor: batch=%v eight=%v twelve=%v", batch, capped, many)
	}
	if end, _ := tt.Find("开始下载"); end != submit {
		t.Errorf("submit moved as links were pasted: %v -> %v", submit, end)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	if links() != empty || tt.HasText("已输入") {
		t.Error("clearing the draft should restore the compact editor")
	}
}

func TestComposeDirectoryPickerBelongsToThePath(t *testing.T) {
	initial, chosen := t.TempDir(), t.TempDir()
	var requested string
	v := presentation.New(nil, presentation.Actions{PickDirectory: func(current string, done func(string, error)) {
		requested = current
		done(chosen, nil)
	}})
	v.Ready = true
	v.Snapshot.Directory = initial
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	picker, ok := tt.AccessByRole(ui.RoleButton, "选择保存目录")
	path, hasPath := tt.AccessByRole(ui.RoleTextField, "保存目录")
	if !ok || !hasPath {
		t.Fatal("missing inline directory chooser or editable path")
	}
	button, field := picker.Bounds, path.Bounds
	if button.X+button.W > field.X || button.Y+button.H/2 != field.Y+field.H/2 {
		t.Fatalf("picker %v should lead the path %v on the same row", button, field)
	}
	links, _ := tt.Find("下载链接")
	submit, ok := tt.Find("开始下载")
	if !ok || button.X != links.X || submit.X+submit.W != links.X+links.W ||
		submit.Y+submit.H/2 != field.Y+field.H/2 || submit.X-(field.X+field.W) != 12 {
		t.Errorf("footer alignment: picker %v, path %v, submit %v, editor %v", button, field, submit, links)
	}
	if _, ok := tt.Find("取消"); ok {
		t.Error("compose still shows a cancel action")
	}
	if err := tt.Click("选择保存目录"); err != nil {
		t.Fatal(err)
	}
	if requested != initial || v.Directory != chosen || !v.DialogOpen {
		t.Fatal("directory selection did not update the current draft")
	}
	if err := tt.Click("保存目录"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(initial)
	if v.Directory != initial {
		t.Fatal("typed path was not retained")
	}
}

func TestRenderComposeDestinationAbbreviatesOnlyTheDefaultFolder(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.DialogOpen = true, true
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true}})
	render := func(head, middle, tail string, isDefault bool) []byte {
		v.Directory = head + strings.Repeat("/documents", 6) + middle + strings.Repeat("/documents", 6) + tail
		if isDefault {
			v.Snapshot.Directory = v.Directory
		}
		tt.Frame()
		field, ok := tt.AccessByRole(ui.RoleTextField, "保存目录")
		if !ok || field.Value != v.Directory {
			t.Fatal("the directory field must retain the full path")
		}
		r, im := field.Bounds, tt.Image()
		var pixels []byte
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			pixels = append(pixels, im.Pix[im.PixOffset(int(r.X), y):im.PixOffset(int(r.X+r.W), y)]...)
		}
		return pixels
	}
	base := render("/Users", "/A", "/Photos", false)
	if !bytes.Equal(base, render("/Users", "/B", "/Photos", false)) {
		t.Error("the middle of the long path should be elided")
	}
	if bytes.Equal(base, render("/Media", "/A", "/Photos", false)) || bytes.Equal(base, render("/Users", "/A", "/Videos", false)) {
		t.Error("both ends of the long path should remain visible")
	}
	defaultFolder := render("/Users", "/A", "/Photos", true)
	if !bytes.Equal(defaultFolder, render("/Media", "/B", "/Photos", true)) ||
		bytes.Equal(defaultFolder, render("/Media", "/B", "/Videos", true)) {
		t.Error("the resting default destination should paint only the final folder name")
	}
}

func TestComposeReservesLeadingToolbarSpaceAndNamesTheDefaultFolder(t *testing.T) {
	for _, recent := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		dir := filepath.Join(t.TempDir(), "Desktop")
		v.Snapshot.Directory = dir
		if recent {
			v.Snapshot.Preferences.RecentDirectories[0] = dir
		}
		v.OpenNew()
		tt := interactionScene(v, 800, 560)
		panel, ok := tt.AccessByRole(ui.RoleDialog, "新建下载")
		field, links := tt.AccessByRole(ui.RoleTextField, "下载链接")
		advanced, tools := tt.AccessByRole(ui.RoleButton, "高级选项")
		if !ok || !links || !tools || !tt.Focused("下载链接") {
			t.Fatal("missing focused link editor or advanced options")
		}
		right := func(r ui.Rect) float32 { return r.X + r.W }
		if advanced.Bounds.Y < panel.Bounds.Y || advanced.Bounds.Y+advanced.Bounds.H > field.Bounds.Y ||
			right(advanced.Bounds) != right(field.Bounds) || advanced.Bounds.X-field.Bounds.X < field.Bounds.W/2 {
			t.Errorf("advanced belongs at the top right, leaving space for future source tabs: %v, editor %v", advanced.Bounds, field.Bounds)
		}
		if _, heading := tt.AccessByRole(ui.RoleText, "新建下载"); heading || tt.HasText("保存到") {
			t.Error("redundant compose labels are still visible")
		}
		if _, shown := tt.Find("Desktop"); !shown {
			t.Error("the default folder should show its name beside the folder icon")
		}
		if err := tt.Click("保存目录"); err != nil {
			t.Fatal(err)
		}
		path, ok := tt.AccessByRole(ui.RoleTextField, "保存目录")
		if !ok || path.Value != dir || !tt.Focused("保存目录") {
			t.Fatal("the full path should still be editable")
		}
	}
}
