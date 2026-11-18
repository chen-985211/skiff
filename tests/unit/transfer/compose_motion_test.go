package transfer_test

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation"
)

func TestSheetExpandsFromButtonAndReversesInPlace(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.URLs = "https://example.org/draft.zip"
	tt := ui.NewTester(v.Render, 1000, 640)
	tt.Advance(time.Second)
	trigger, ok := tt.AccessByRole(ui.RoleButton, "新建下载")
	if !ok {
		t.Fatal("missing new download button")
	}
	sheet := func() ui.Rect {
		t.Helper()
		node, found := tt.AccessByRole(ui.RoleDialog, "")
		if !found {
			t.Fatal("sheet disappeared during the transition")
		}
		return node.Bounds
	}
	near := func(a, b ui.Rect) bool {
		for _, delta := range []float32{a.X - b.X, a.Y - b.Y, a.W - b.W, a.H - b.H} {
			if math.Abs(float64(delta)) > 1 {
				return false
			}
		}
		return true
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	if r := sheet(); !near(r, trigger.Bounds) {
		t.Errorf("opening frame %v, want the pressed button %v", r, trigger.Bounds)
	}
	tt.Advance(150 * time.Millisecond)
	tt.Frame()
	expanding := sheet()
	tt.Key(0, ui.KeyEscape)
	if v.DialogOpen || !near(sheet(), expanding) {
		t.Fatal("dismissal jumped away from the current outline")
	}
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	returning := sheet()
	if returning.W >= expanding.W || returning.H >= expanding.H || returning.W <= trigger.Bounds.W {
		t.Fatalf("expected a partially contracted outline, got %v -> %v", expanding, returning)
	}
	tt.Key(ui.Cmd, ui.KeyN)
	if !v.DialogOpen || !near(sheet(), returning) {
		t.Fatalf("reopening jumped or ignored the shortcut: open=%v, %v -> %v", v.DialogOpen, returning, sheet())
	}
	tt.Advance(time.Second)
	if r := sheet(); r.W <= expanding.W || v.URLs != "https://example.org/draft.zip" {
		t.Fatalf("did not finish opening the same draft: %v %q", r, v.URLs)
	}
	tt.ClickAt(40, 80)
	if v.DialogOpen {
		t.Fatal("click outside did not dismiss the form")
	}
	tt.Advance(time.Second)
	if _, open := tt.AccessByRole(ui.RoleDialog, ""); open {
		t.Fatal("dismissed sheet still intercepts input")
	}
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
}

func TestSheetExpandsAgainWhenReturningButtonIsClicked(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTester(v.Render, 800, 560)
	button, _ := tt.Find("新建下载")
	clickButton := func() { tt.ClickAt(button.X+button.W/2, button.Y+button.H/2) }
	clickButton()
	tt.Advance(80 * time.Millisecond)
	clickButton()
	if v.DialogOpen {
		t.Fatal("a second click on the expanding button did not reverse it")
	}
	tt.Advance(65 * time.Millisecond)
	clickButton()
	if !v.DialogOpen {
		t.Fatal("clicking the returning button did not open it again")
	}
	tt.Type("https://example.org/resume.zip")
	if v.URLs != "https://example.org/resume.zip" {
		t.Fatal("link editor lost focus after reversing")
	}
}

func TestSheetDismissalStartsWithTheVisibleForm(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.DialogOpen = true, true
	tt := ui.NewTester(v.Render, 800, 560)
	tt.Advance(time.Second)
	button, ok := tt.Find("开始下载")
	if !ok {
		t.Fatal("missing submit action")
	}
	x, y := int(button.X+8), int(button.Y+button.H/2)
	before := tt.Image().RGBAAt(x, y)
	tt.Key(0, ui.KeyEscape)
	if after := tt.Image().RGBAAt(x, y); before != after {
		t.Fatalf("action flashed on dismissal before the sheet moved: %v -> %v", before, after)
	}
}
