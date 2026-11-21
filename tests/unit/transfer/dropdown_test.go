package transfer_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
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
	dns, _ := tt.AccessByRole(ui.RoleMenuButton, "DNS · 自动")
	click("DNS · 自动")
	menu, shown := tt.AccessByRole(ui.RoleMenu, "")
	if !shown || menu.Bounds.W+1 < dns.Bounds.W {
		t.Fatalf("menu %.1f narrower than field %.1f", menu.Bounds.W, dns.Bounds.W)
	}
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

func TestRenderTaskRateDropdownUsesCardMaterial(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dark  bool
		prefs ui.Preferences
	}{
		{name: "light"}, {name: "dark", dark: true},
		{name: "opaque", prefs: ui.Preferences{ReduceTransparency: true}},
		{name: "high contrast", dark: true, prefs: ui.Preferences{HighContrast: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var faces [2][]color.RGBA
			for i, id := range []string{"first", "other-task"} {
				v := presentation.New(nil, presentation.Actions{})
				v.Ready = true
				v.Snapshot.Preferences.StaticBackground = true
				v.Snapshot.Tasks = []application.TaskDTO{{ID: id, Name: "one.zip", Status: "paused"}}
				tt := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{Dark: tc.dark, Preferences: tc.prefs})
				if err := tt.Click("查看详情 one.zip"); err != nil {
					t.Fatal(err)
				}
				tt.Move(2, 2)
				tt.Advance(time.Second)
				r := sourceBounds(t, tt, ui.RoleMenuButton, "任务限速 · 不限速")
				// The straight top edge and the strip just inside it contain no glyphs.
				stripe := func(inset int) []color.RGBA {
					im := tt.Image()
					var row []color.RGBA
					for x := int(r.X) + 20; x < int(r.X+r.W)-20; x += 3 {
						row = append(row, im.RGBAAt(x, int(math.Ceil(float64(r.Y)))+inset))
					}
					return row
				}
				faces[i] = stripe(4)
				if !tc.prefs.ReduceTransparency && !tc.prefs.HighContrast && changedFraction(stripe(0), faces[i], 10) > .3 {
					t.Error("idle trigger has a hard contrasting outline")
				}
				tt.Move(r.X+r.W/2, r.Y+r.H/2)
				tt.Advance(time.Second)
				if changedFraction(faces[i], stripe(4), 3) < .5 {
					t.Error("hover should change the trigger surface")
				}
			}
			translucent := !tc.prefs.ReduceTransparency && !tc.prefs.HighContrast
			if change := changedFraction(faces[0], faces[1], 2); translucent && change < .25 || !translucent && change > 0 {
				t.Errorf("fraction of trigger surface reflecting the card: %.2f", change)
			}
		})
	}
}

func TestRenderDropdownsShareTriggerAndMenuFeedback(t *testing.T) {
	for _, dark := range []bool{false, true} {
		var hoveredTriggers []color.RGBA
		for _, task := range []bool{false, true} {
			v := presentation.New(nil, presentation.Actions{})
			v.Ready = true
			v.Snapshot.Preferences.StaticBackground = true
			v.Snapshot.Tasks = []application.TaskDTO{{ID: "one", Name: "one.zip", Status: "paused"}}
			tt := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{Dark: dark, Preferences: ui.Preferences{ReduceTransparency: true}})
			page, label := "设置", "全局限速 · 不限速"
			if task {
				page, label = "查看详情 one.zip", "任务限速 · 不限速"
			}
			if err := tt.Click(page); err != nil {
				t.Fatal(err)
			}
			tt.Move(2, 2)
			tt.Advance(time.Second)
			trigger := sourceBounds(t, tt, ui.RoleMenuButton, label)
			triggerFace := func() color.RGBA {
				return tt.Image().RGBAAt(int(trigger.X+trigger.W/2), int(trigger.Y+5))
			}
			idle := triggerFace()
			tt.Move(trigger.X+trigger.W/2, trigger.Y+trigger.H/2)
			tt.Advance(time.Second)
			hoveredTriggers = append(hoveredTriggers, triggerFace())
			if idle == triggerFace() {
				t.Errorf("%s dark=%v: hovering leaves the trigger unchanged", label, dark)
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Advance(time.Second)
			checked := sourceBounds(t, tt, ui.RoleMenuItemCheckBox, "不限速")
			other := sourceBounds(t, tt, ui.RoleMenuItemCheckBox, "512 KiB/s")
			face := func(b ui.Rect) color.RGBA {
				return tt.Image().RGBAAt(int(b.X+b.W-18), int(b.Y+b.H/2))
			}
			base := face(other)
			tt.Move(other.X+other.W/2, other.Y+other.H/2)
			tt.Advance(time.Second)
			hover := face(other)
			tt.Key(0, ui.KeyHome)
			tt.Advance(time.Second)
			key := face(checked)
			// The selected row keeps a legible check mark and caption on the highlight.
			im := tt.Image()
			for _, span := range [][2]int{{12, 25}, {36, 73}} {
				var ink int
				for y := int(checked.Y + 4); y < int(checked.Y+checked.H-4); y++ {
					for x := int(checked.X) + span[0]; x < int(checked.X)+span[1]; x++ {
						diff := int(key.R) - int(im.RGBAAt(x, y).R)
						if dark {
							diff = -diff
						}
						if diff > 75 {
							ink++
						}
					}
				}
				if ink < 4 {
					t.Errorf("%s dark=%v: missing legible ink in row columns %v", label, dark, span)
				}
			}
			// Keyboard and pointer use the same highlight. Its shade should
			// be distinct from an idle row without reversing the menu's polarity.
			delta := math.Abs(float64(hover.R) - float64(base.R))
			if hover != key || delta < 10 || delta > 85 || face(other) != base {
				t.Errorf("%s dark=%v: idle %v, hovered %v, keyboard %v, previous row %v", label, dark, base, hover, key, face(other))
			}
		}
		if hoveredTriggers[0] != hoveredTriggers[1] {
			t.Errorf("dark=%v: settings and task triggers have different hover colors: %v", dark, hoveredTriggers)
		}
	}
}

func TestRenderChangingDownloadLimitKeepsUnrelatedControlsStable(t *testing.T) {
	for _, tc := range []struct{ page, trigger, neighbor string }{
		{"设置", "全局限速", "更改默认保存目录"},
		{"查看详情 one.zip", "任务限速", "新建下载"},
	} {
		t.Run(tc.trigger, func(t *testing.T) {
			s := service(t, &fakeEngine{}, &fakeRepo{})
			if _, err := s.Add(t.Context(), "https://example.test/one.zip", t.TempDir()); err != nil {
				t.Fatal(err)
			}
			if err := s.SetStaticBackground(t.Context(), true); err != nil {
				t.Fatal(err)
			}
			dispatch := make(chan func(), 2)
			v := presentation.New(s, presentation.Actions{
				Dispatch:      func(f func()) { dispatch <- f },
				PickDirectory: func(string, func(string, error)) {},
			})
			v.Snapshot = s.Snapshot()
			tt := ui.NewTester(v.Render, 1020, 680)
			chooseSource(t, tt, tc.page)
			tt.Move(2, 2)
			tt.Advance(time.Second)
			b := sourceBounds(t, tt, ui.RoleButton, tc.neighbor)
			area := image.Rect(int(b.X+5), int(b.Y+5), int(b.X+b.W-5), int(b.Y+b.H-5))
			base := renderedRegion(tt, area)
			check := func(phase string) {
				t.Helper()
				if !bytes.Equal(base, renderedRegion(tt, area)) {
					t.Errorf("%s: %s repainted differently", phase, tc.neighbor)
				}
				if n, ok := tt.AccessByRole(ui.RoleButton, tc.neighbor); !ok || n.Disabled {
					t.Errorf("%s: %s unavailable", phase, tc.neighbor)
				}
			}
			chooseSource(t, tt, tc.trigger+" · 不限速")
			tt.Advance(80 * time.Millisecond)
			check("opening")
			tt.Advance(time.Second)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEscape)
			tt.Advance(80 * time.Millisecond)
			check("dismissal")
			tt.Advance(time.Second)
			chooseSource(t, tt, tc.trigger+" · 不限速")
			chooseSource(t, tt, "1 MiB/s")
			check("selection")
			tt.Advance(80 * time.Millisecond)
			check("saving")
			// A newer observation can arrive while the UI callback is queued.
			v.Snapshot.Tasks = append([]application.TaskDTO(nil), v.Snapshot.Tasks...)
			v.Snapshot.Tasks[0].Connections = 7
			select {
			case complete := <-dispatch:
				complete()
			case <-time.After(2 * time.Second):
				t.Fatal("missing rate completion")
			}
			tt.Advance(time.Second)
			check("saved")
			if v.Snapshot.Tasks[0].Connections != 7 {
				t.Fatal("rate completion replaced a newer task observation")
			}
			sourceBounds(t, tt, ui.RoleMenuButton, tc.trigger+" · 1 MiB/s")
		})
	}
}

func TestRateSelectionsKeepLatestValueAndRollbackInPlace(t *testing.T) {
	repo := &fakeRepo{}
	s := service(t, &fakeEngine{}, repo)
	completions := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(f func()) { completions <- f }})
	v.Snapshot = s.Snapshot()
	tt := interactionScene(v, 1020, 680)
	apply := func() {
		t.Helper()
		select {
		case f := <-completions:
			f()
			tt.Frame()
		case <-time.After(2 * time.Second):
			t.Fatal("missing download limit save")
		}
	}
	chooseSource(t, tt, "设置")
	chooseSource(t, tt, "全局限速 · 不限速")
	chooseSource(t, tt, "1 MiB/s")
	chooseSource(t, tt, "全局限速 · 1 MiB/s")
	chooseSource(t, tt, "2 MiB/s")
	apply()
	sourceBounds(t, tt, ui.RoleMenuButton, "全局限速 · 2 MiB/s")
	apply()
	if s.Snapshot().Preferences.DownloadLimit != 2<<20 {
		t.Fatal("latest rate was not saved")
	}
	repo.saveErr = errors.New("disk full")
	chooseSource(t, tt, "全局限速 · 2 MiB/s")
	chooseSource(t, tt, "不限速")
	apply()
	sourceBounds(t, tt, ui.RoleMenuButton, "全局限速 · 2 MiB/s")
	if !tt.HasText("保存任务失败：disk full") {
		t.Error("missing local save error")
	}
	if _, ok := tt.AccessByRole(ui.RoleDialog, "全局限速"); ok {
		t.Error("failure unexpectedly opened a modal")
	}
}
