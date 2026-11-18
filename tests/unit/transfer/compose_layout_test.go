package transfer_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation"
)

func TestComposeLinkFieldGrowsWithinTheAnchoredSheet(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
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
	if empty.H >= 80 {
		t.Errorf("empty editor takes %.1f pt", empty.H)
	}
	paste := func(n int) {
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(strings.TrimSuffix(strings.Repeat("https://example.org/file.zip\n", n), "\n"))
	}
	paste(4)
	batch := links()
	count, ok := tt.Find("已输入 4 行")
	if batch.H <= empty.H || !ok || count.Y+count.H > batch.Y {
		t.Errorf("batch count should sit above a growing editor: count=%v empty=%v batch=%v", count, empty, batch)
	}
	paste(6)
	capped := links()
	paste(12)
	if many := links(); many.H != capped.H || many.H <= batch.H || many.Y < 0 {
		t.Errorf("large batches should scroll within the editor: batch=%v six=%v twelve=%v", batch, capped, many)
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
	tt := ui.NewTester(v.Render, 800, 560)
	picker, ok := tt.AccessByRole(ui.RoleButton, "选择保存目录")
	path, hasPath := tt.AccessByRole(ui.RoleTextField, "保存目录")
	if !ok || !hasPath {
		t.Fatal("missing inline directory chooser or editable path")
	}
	button, field := picker.Bounds, path.Bounds
	if button.X+button.W > field.X || button.Y+button.H/2 != field.Y+field.H/2 {
		t.Fatalf("picker %v should lead the path %v on the same row", button, field)
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
