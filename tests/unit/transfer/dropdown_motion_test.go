package transfer_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

func TestRenderDropdownHighlightFollowsAndReversesBetweenRows(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 900, 650, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true, ReduceMotion: reduced}})
		chooseSource(t, tt, "设置")
		tt.Advance(time.Second)
		chooseSource(t, tt, "全局限速 · 不限速")
		tt.Key(0, ui.KeyHome)
		tt.Advance(time.Second)
		a := sourceBounds(t, tt, ui.RoleMenuItemCheckBox, "不限速")
		b := sourceBounds(t, tt, ui.RoleMenuItemCheckBox, "512 KiB/s")
		x := int(a.X + a.W - 22)
		face := tt.Image()
		ink := int(face.RGBAAt(x, int(a.Y+a.H/2)).R)
		idle := int(face.RGBAAt(x, int(b.Y+b.H/2)).R)
		if idle-ink < 12 {
			t.Fatal("highlight missing at rest")
		}
		// Locate the solid vertical band in a glyph-free strip across both rows.
		band := func() (top, height int) {
			im := tt.Image()
			for y := int(a.Y); y < int(b.Y+b.H); y++ {
				if int(im.RGBAAt(x, y).R) < ink+(idle-ink)/4 {
					if height == 0 {
						top = y
					}
					height++
				}
			}
			return
		}
		start, height := band()
		tt.Move(b.X+b.W/2, b.Y+b.H/2)
		first, _ := band()
		tt.Advance(55 * time.Millisecond)
		// Advance can stop between refreshes. Render at the timestamp of
		// the next input so the comparison measures retargeting alone.
		tt.Frame()
		mid, midHeight := band()
		tt.Key(0, ui.KeyHome)
		reverse, _ := band()
		tt.Advance(time.Second)
		rest, _ := band()
		if reduced {
			if first < int(b.Y)-1 || mid != first || reverse != start || rest != start {
				t.Errorf("reduced motion positions %d → %d → %d → %d → %d", start, first, mid, reverse, rest)
			}
		} else if first != start || mid <= start+2 || mid >= int(b.Y)-2 || midHeight < height-2 || midHeight > height+2 || reverse < mid-1 || reverse > mid+1 || rest != start {
			t.Errorf("highlight positions %d → %d → %d → %d → %d, band height %d → %d", start, first, mid, reverse, rest, height, midHeight)
		}
	}
}

func TestRatePresetSelectionKeepsTriggerInPlace(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	if _, err := s.Add(t.Context(), "https://example.test/one.zip", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ page, scope string }{{"设置", "全局限速"}, {"查看详情 one.zip", "任务限速"}} {
		done := make(chan func(), 1)
		v := presentation.New(s, presentation.Actions{Dispatch: func(f func()) { done <- f }})
		v.Snapshot = s.Snapshot()
		tt := interactionScene(v, 800, 560)
		chooseSource(t, tt, tc.page)
		before := sourceBounds(t, tt, ui.RoleMenuButton, tc.scope+" · 不限速")
		chooseSource(t, tt, tc.scope+" · 不限速")
		chooseSource(t, tt, "512 KiB/s")
		after := sourceBounds(t, tt, ui.RoleMenuButton, tc.scope+" · 512 KiB/s")
		if before != after {
			t.Errorf("%s moved when its caption changed: %v → %v", tc.scope, before, after)
		}
		select {
		case f := <-done:
			f()
		case <-time.After(2 * time.Second):
			t.Fatal("save did not complete")
		}
	}
}
