package transfer_test

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

func TestSettingsDropdownPicksWithKeysAndDismissesInsideDialog(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := interactionScene(v, 800, 560)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	click("设置")
	click("代理与 DNS")
	click("DNS · 自动")
	if _, ok := tt.AccessByRole(ui.RoleMenuItemCheckBox, "系统解析"); !ok {
		t.Fatal("missing in-window DNS choices")
	}
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	if _, ok := tt.AccessByRole(ui.RoleMenuButton, "DNS · 系统解析"); !ok {
		t.Fatal("keyboard selection did not update DNS")
	}
	// Focus should return to the same trigger even though its value changed.
	tt.Key(0, ui.KeyDown)
	if _, ok := tt.AccessByRole(ui.RoleMenuItemCheckBox, "自动"); !ok {
		t.Fatal("could not reopen the updated trigger from the keyboard")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(time.Second)
	if _, ok := tt.AccessByRole(ui.RoleDialog, ""); !ok {
		t.Fatal("Escape closed the containing preferences dialog")
	}
	if _, ok := tt.AccessByRole(ui.RoleMenu, ""); ok {
		t.Fatal("menu is still actionable after Escape")
	}
}

func TestRenderDropdownPaintsIntermediateFramesInBothDirections(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTesterWith(v.Render, 900, 650, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	frame := func() *image.RGBA {
		im := tt.Image()
		cp := image.NewRGBA(im.Bounds())
		copy(cp.Pix, im.Pix)
		return cp
	}
	base := frame()
	if err := tt.Click("全局限速 · 不限速"); err != nil {
		t.Fatal(err)
	}
	first := frame()
	tt.Advance(75 * time.Millisecond)
	opening := frame()
	tt.Advance(time.Second)
	open := frame()
	menu, ok := tt.AccessByRole(ui.RoleMenu, "")
	if !ok {
		t.Fatal("missing menu")
	}
	b := menu.Bounds
	area := image.Rect(int(b.X+6), int(b.Y+6), int(b.X+b.W-6), int(b.Y+b.H-6))
	distance := func(a, b *image.RGBA) float64 {
		var sum float64
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				sum += math.Abs(float64(a.RGBAAt(x, y).R) - float64(b.RGBAAt(x, y).R))
			}
		}
		return sum / float64(area.Dx()*area.Dy())
	}
	tt.Key(0, ui.KeyEscape)
	last := frame()
	tt.Advance(75 * time.Millisecond)
	closing := frame()
	tt.Advance(time.Second)
	closed := frame()
	full := distance(base, open)
	if full < 3 || distance(base, first) > full*.15 || distance(open, last) > full*.15 || distance(closed, base) > full*.1 {
		t.Errorf("menu jumped at its boundary: full %.1f first %.1f last %.1f closed %.1f", full, distance(base, first), distance(open, last), distance(closed, base))
	}
	for _, im := range []*image.RGBA{opening, closing} {
		if distance(im, base) < full*.08 || distance(im, open) < full*.08 {
			t.Errorf("missing intermediate paint: base %.1f open %.1f, total %.1f", distance(im, base), distance(im, open), full)
		}
	}
}

func TestRecentDirectoryMenuFitsAboveComposerAndUpdatesDraft(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	a, b := t.TempDir(), t.TempDir()
	v.Snapshot.Directory = a
	v.Snapshot.Preferences.RecentDirectories = [5]string{a, b}
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("选择保存目录"); err != nil {
		t.Fatal(err)
	}
	menu, ok := tt.AccessByRole(ui.RoleMenu, "")
	trigger, found := tt.Find("选择保存目录")
	if !ok || !found || menu.Bounds.Y+menu.Bounds.H > trigger.Y+1 || menu.Bounds.X < 0 || menu.Bounds.X+menu.Bounds.W > 800 {
		t.Fatalf("recent directories should open above their trigger: %v / %v", menu.Bounds, trigger)
	}
	if err := tt.Click(b); err != nil {
		t.Fatal(err)
	}
	if v.Directory != b || !v.DialogOpen {
		t.Fatal("choosing a recent directory lost the download draft")
	}
}
