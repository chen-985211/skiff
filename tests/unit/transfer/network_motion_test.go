package transfer_test

import (
	"bytes"
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

func TestRenderManualProxyRevealsWithoutAnExtraGap(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	for _, label := range []string{"设置", "代理与 DNS"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
	}
	dns := func() float32 {
		n, ok := tt.Find("DNS")
		if !ok {
			t.Fatal("missing DNS row")
		}
		return n.Y
	}
	before := dns()
	if err := tt.Click("手动代理"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if first := dns(); math.Abs(float64(first-before)) > 1 {
		t.Fatalf("DNS jumped on the first frame: %.1f -> %.1f", before, first)
	}
	tt.Advance(70 * time.Millisecond)
	tt.Frame()
	mid := dns()
	if err := tt.Click("直连"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if reversed := dns(); math.Abs(float64(mid-reversed)) > 1 {
		t.Fatalf("reversing jumped: %.1f -> %.1f", mid, reversed)
	}
	tt.Advance(time.Second)
	if dns() != before || mid <= before+1 {
		t.Fatalf("manual editor never travelled continuously: %.1f %.1f %.1f", before, mid, dns())
	}
}

func TestRenderPreferencesPanelEntersAndLeavesContinuously(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	sample := func() *image.RGBA {
		im := tt.Image()
		frame := image.NewRGBA(im.Bounds())
		copy(frame.Pix, im.Pix)
		return frame
	}
	base := sample()
	if err := tt.Click("代理与 DNS"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	first := sample()
	tt.Advance(75 * time.Millisecond)
	tt.Frame()
	partial := sample()
	tt.Advance(time.Second)
	tt.Frame()
	dialog, ok := tt.AccessByRole(ui.RoleDialog, "")
	if !ok {
		t.Fatal("missing network panel")
	}
	area := image.Rect(int(dialog.Bounds.X+12), int(dialog.Bounds.Y+10), int(dialog.Bounds.X+dialog.Bounds.W-12), int(dialog.Bounds.Y+dialog.Bounds.H-10))
	dist := func(a, b *image.RGBA) float64 {
		var d float64
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				d += math.Abs(float64(a.RGBAAt(x, y).R) - float64(b.RGBAAt(x, y).R))
			}
		}
		return d / float64(area.Dx()*area.Dy())
	}
	full := sample()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	leaving := sample()
	tt.Advance(75 * time.Millisecond)
	tt.Frame()
	closing := sample()
	tt.Advance(time.Second)
	tt.Frame()
	end := sample()
	total := dist(base, full)
	entryJumps := dist(base, first) > total*.12
	exitJumps := dist(full, leaving) > total*.12
	intermediate := func(frame *image.RGBA) bool {
		return dist(base, frame) > total*.08 && dist(full, frame) > total*.08
	}
	if total < 3 || entryJumps || exitJumps || !intermediate(partial) || !intermediate(closing) || dist(end, closing) < total*.08 {
		t.Fatalf("panel must paint intermediate content both ways: total=%.1f enter=%.1f/mid %.1f exit=%.1f/mid %.1f", total, dist(base, first), dist(base, partial), dist(full, leaving), dist(full, closing))
	}
}

func TestDownloadPreferencesCanReopenDuringDismissal(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true, ReduceMotion: reduce}})
		for _, label := range []string{"设置", "性能与磁盘"} {
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Advance(time.Second)
		}
		tt.ClickAt(20, 30)
		tt.Frame()
		if _, ok := tt.AccessByRole(ui.RoleDialog, ""); ok {
			t.Fatal("closing panel still owns focus")
		}
		tt.Advance(60 * time.Millisecond)
		if err := tt.Click("性能与磁盘"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if _, ok := tt.AccessByRole(ui.RoleButton, "恢复默认"); !ok {
			t.Fatal("cannot reopen the closing editor")
		}
		if reduce {
			first := tt.Image()
			tt.Advance(time.Second)
			if !bytes.Equal(first.Pix, tt.Image().Pix) {
				t.Fatal("reduced motion left a transition running")
			}
		}
	}
}
