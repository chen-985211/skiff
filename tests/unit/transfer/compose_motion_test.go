package transfer_test

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation"
)

func TestRenderSheetExpandsWithoutShiftingButtonLabelOnClick(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.Snapshot.Preferences.StaticBackground = true, true
	tt := ui.NewTester(v.Render, 1020, 680)
	tt.Advance(time.Second)
	button, ok := tt.Find("新建下载")
	if !ok {
		t.Fatal("missing new download button")
	}
	// Measure just the light ink inside the pill, clear of its rounded rim.
	inkBounds := func() image.Rectangle {
		im := tt.Image()
		var ink image.Rectangle
		for y := int(button.Y + 6); y < int(button.Y+button.H-6); y++ {
			for x := int(button.X + 11); x < int(button.X+button.W-11); x++ {
				if im.RGBAAt(x, y).R > 175 {
					ink = ink.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
		return ink
	}
	idle := inkBounds()
	if err := tt.Click("新建下载"); err != nil {
		t.Fatal(err)
	}
	if first := inkBounds(); idle.Empty() || first != idle {
		t.Fatalf("label shifted on the opening frame: %v -> %v", idle, first)
	}
}

func TestSheetExpandsAndReversesWithOneHeightTarget(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.Snapshot.Preferences.StaticBackground = true, true
	v.URLs = "one\ntwo\nthree\nfour\nfive\nsix"
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	button, _ := tt.Find("新建下载")
	tt.Key(ui.Cmd, ui.KeyN)
	tt.Advance(time.Second)
	open, ok := tt.AccessByRole(ui.RoleDialog, "")
	if !ok || open.Bounds.W <= button.W {
		t.Fatal("missing expanded sheet")
	}
	tt.Key(0, ui.KeyEscape)
	for frame := 0; frame < 72; frame++ {
		if frame == 5 {
			tt.Key(ui.Cmd, ui.KeyN)
		}
		tt.Advance(12 * time.Millisecond)
		r, ok := tt.AccessByRole(ui.RoleDialog, "")
		if !ok {
			t.Fatal("sheet disappeared during interruption")
		}
		progress := (r.Bounds.W - button.W) / (open.Bounds.W - button.W)
		want := button.H + progress*(open.Bounds.H-button.H)
		if math.Abs(float64(r.Bounds.H-want)) > .6 {
			t.Fatalf("frame %d: width %.2f implies height %.2f, got %.2f", frame, r.Bounds.W, want, r.Bounds.H)
		}
	}
}

func TestSheetExpandsFromButtonAndReversesInPlace(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.URLs = "https://example.org/draft.zip"
	tt := ui.NewTesterWith(v.Render, 1000, 640, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
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
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
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

func TestRenderSheetDismissalStartsWithTheVisibleForm(t *testing.T) {
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
