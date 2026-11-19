package transfer_test

import (
	"bytes"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestSettingsListAlignsRowsAndKeepsFullPathAccessible(t *testing.T) {
	for _, width := range []int{800, 1020} {
		v := presentation.New(nil, presentation.Actions{PickDirectory: func(string, func(string, error)) {}})
		v.Ready = true
		v.Snapshot.Directory = "/Users/developer/Downloads/archives/2026/a-long-folder-name"
		tt := ui.NewTesterWith(v.Render, width, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true}})
		if err := tt.Click("设置"); err != nil {
			t.Fatal(err)
		}
		dir, ok := tt.AccessByRole(ui.RoleButton, "更改默认保存目录")
		flow, switchOK := tt.AccessByRole(ui.RoleSwitch, "背景流光")
		theme, groupOK := tt.AccessByRole(ui.RoleRadioGroup, "主题")
		if !ok || !switchOK || !groupOK || dir.Description != v.Snapshot.Directory ||
			dir.Bounds.H > 95 || theme.Bounds.Y < dir.Bounds.Y+dir.Bounds.H || flow.Bounds.Y <= theme.Bounds.Y+theme.Bounds.H ||
			math.Abs(float64(theme.Bounds.X+theme.Bounds.W-flow.Bounds.X-flow.Bounds.W)) > 2 ||
			!flow.Disabled || !tt.HasText("默认保存目录") {
			t.Fatalf("width %d: directory %v, theme %v, switch %v", width, dir.Bounds, theme.Bounds, flow.Bounds)
		}
	}
}

// Measure the rendered track, so both a snapped thumb and a one-frame color
// change can be detected without exposing animation state to tests.
func TestRenderSettingsSwitchReversesInFlight(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	control, ok := tt.AccessByRole(ui.RoleSwitch, "背景流光")
	if !ok {
		t.Fatal("missing switch")
	}
	rect := control.Bounds
	sample := func() []float64 {
		var pixels []float64
		img := tt.Image()
		y := int(rect.Y + rect.H/2)
		for x := int(rect.X) + 2; x < int(rect.X+rect.W)-2; x++ {
			col := img.RGBAAt(x, y)
			pixels = append(pixels, float64(col.R), float64(col.G), float64(col.B))
		}
		return pixels
	}
	distance := func(a, b []float64) float64 {
		var d float64
		for i := range a {
			d += math.Abs(a[i] - b[i])
		}
		return d
	}
	start := sample()
	v.Snapshot.Preferences.StaticBackground = true
	tt.Frame()
	tt.Advance(55 * time.Millisecond)
	tt.Frame() // Sample at the exact time of the upcoming reversal.
	mid := sample()
	v.Snapshot.Preferences.StaticBackground = false
	tt.Frame()
	reversed := sample()
	tt.Advance(time.Second)
	back := sample()
	v.Snapshot.Preferences.StaticBackground = true
	tt.Frame()
	tt.Advance(time.Second)
	end := sample()
	span := distance(start, end)
	if span < 300 || distance(mid, start) < span*.08 || distance(mid, end) < span*.08 ||
		distance(mid, reversed) > span*.06 || distance(start, back) > span*.01 {
		t.Fatalf("switch %.0f; start-mid %.0f, mid-end %.0f, reversal jump %.0f, return %.0f", span, distance(mid, start), distance(mid, end), distance(mid, reversed), distance(start, back))
	}
}

func TestRenderSettingsSelectionTravelsThroughSidebar(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	choice, _ := tt.AccessByRole(ui.RoleButton, "设置")
	last, _ := tt.AccessByRole(ui.RoleButton, "下载失败")
	x, y := int(choice.Bounds.X+choice.Bounds.W/2), int((last.Bounds.Y+choice.Bounds.Y)/2)
	before := tt.Image().RGBAAt(x, y)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	for range 36 {
		tt.Advance(8 * time.Millisecond)
		if tt.Image().RGBAAt(x, y) != before {
			return
		}
	}
	t.Fatal("selection disappeared from the categories and appeared directly at settings")
}

func TestRenderSettingsThemeFollowsSystemAndRestoresOverride(t *testing.T) {
	repo := &fakeRepo{}
	s := service(t, &fakeEngine{}, repo)
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { done <- fn }})
	v.Snapshot = s.Snapshot()
	// An opaque canvas away from content isolates the application palette.
	tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	canvas := func() int { col := tt.Image().RGBAAt(890, 3); return int(col.R) + int(col.G) + int(col.B) }
	initial := canvas()
	choose := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		select {
		case apply := <-done:
			apply()
			tt.Frame()
		case <-time.After(2 * time.Second):
			t.Fatal("theme was not saved")
		}
	}
	chk := func(label string) {
		t.Helper()
		item, ok := tt.AccessByRole(ui.RoleRadio, label)
		if !ok || !item.Checked {
			t.Fatalf("theme selection missing: %q", label)
		}
	}
	chk("跟随系统")
	choose("深色")
	tt.Advance(85 * time.Millisecond)
	middle := canvas()
	tt.Advance(time.Second)
	dark := canvas()
	if !(initial > middle+15 && middle > dark+15) {
		t.Fatalf("theme jumped: %d, %d, %d", initial, middle, dark)
	}
	if err := tt.Click("全部任务"); err != nil {
		t.Fatal(err)
	}
	if canvas() != dark {
		t.Fatal("theme did not extend to download workspace")
	}
	// Reopen with a light system appearance; the saved dark override wins.
	v = presentation.New(service(t, &fakeEngine{}, repo), presentation.Actions{Dispatch: func(fn func()) { done <- fn }})
	v.Snapshot = v.API.Snapshot()
	tt = ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	chk("深色")
	if canvas() != dark {
		t.Fatal("dark theme lost after restart")
	}
	tt.SetDark(true)
	choose("浅色")
	tt.Advance(time.Second)
	chk("浅色")
	if canvas() != initial {
		t.Fatal("light override ignored")
	}
	choose("跟随系统")
	tt.Advance(time.Second)
	chk("跟随系统")
	if canvas() != dark {
		t.Fatal("system dark was not restored")
	}
	tt.SetDark(false)
	tt.Advance(time.Second)
	if canvas() != initial {
		t.Fatal("did not follow system appearance changes")
	}
}

func TestRenderSettingsSidebarKeepsSelectedLabelAboveHighlight(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Dark: true, Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true}})
	for _, name := range []string{"全部任务", "设置"} {
		if name == "设置" {
			if err := tt.Click(name); err != nil {
				t.Fatal(err)
			}
		}
		item, ok := tt.AccessByRole(ui.RoleButton, name)
		if !ok {
			t.Fatal(name)
		}
		r := item.Bounds
		bright := 0
		img := tt.Image()
		for y := int(r.Y + 6); y < int(r.Y+r.H-6); y++ {
			for x := int(r.X + 32); x < int(r.X+r.W-16); x++ {
				px := img.RGBAAt(x, y)
				if px.R > 165 && px.G > 165 && px.B > 165 {
					bright++
				}
			}
		}
		if bright < 8 {
			t.Errorf("selected %s label is occluded (%d text pixels)", name, bright)
		}
	}
}

func TestSettingsThemeKeepsLatestChoiceDuringSaveAndRevertsOnFailure(t *testing.T) {
	repo := &fakeRepo{}
	s := service(t, &fakeEngine{}, repo)
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { done <- fn }})
	v.Snapshot = s.Snapshot()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	click := func(name string) {
		t.Helper()
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	wait := func() {
		t.Helper()
		select {
		case apply := <-done:
			apply()
			tt.Frame()
		case <-time.After(2 * time.Second):
			t.Fatal("missing theme save")
		}
	}
	click("设置")
	click("深色")
	click("浅色") // The first completion is still waiting for the UI thread.
	wait()
	wait()
	if got := s.Snapshot().Preferences.Theme; got != application.ThemeLight {
		t.Fatalf("saved %q", got)
	}
	repo.saveErr = errors.New("disk full")
	click("深色")
	wait()
	item, _ := tt.AccessByRole(ui.RoleRadio, "浅色")
	if !item.Checked || !tt.HasText("保存任务失败：disk full") {
		t.Fatal("failed save did not restore and explain the previous theme")
	}
	click("全部任务")
	if tt.HasText("保存任务失败：disk full") {
		t.Error("theme save failure escaped the settings page")
	}
}

func TestRenderSettingsFlowSwitchFreezesImmediatelyWithoutDimmingThePage(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{
		Dispatch:      func(f func()) { done <- f },
		PickDirectory: func(string, func(string, error)) {},
	})
	v.Snapshot = s.Snapshot()
	tt := ui.NewTester(v.Render, 800, 560)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	click("设置")
	tt.Advance(time.Second)
	click("背景流光")
	dir, _ := tt.AccessByRole(ui.RoleButton, "更改默认保存目录")
	sw, _ := tt.AccessByRole(ui.RoleSwitch, "背景流光")
	if dir.Disabled || sw.Disabled || sw.Checked {
		t.Errorf("while saving: directory disabled=%t; switch disabled=%t, checked=%t", dir.Disabled, sw.Disabled, sw.Checked)
	}
	sample := func() []byte {
		img := tt.Image()
		var line []byte
		for x := 20; x < 780; x += 12 {
			line = append(line, img.RGBAAt(x, 430).R)
		}
		return line
	}
	frozen := sample()
	tt.Advance(500 * time.Millisecond)
	if !bytes.Equal(frozen, sample()) {
		t.Error("the backdrop kept moving while the off preference was being saved")
	}
	select {
	case apply := <-done:
		apply()
	case <-time.After(2 * time.Second):
		t.Fatal("background preference was not saved")
	}
	tt.Frame()
	if !bytes.Equal(frozen, sample()) {
		t.Error("saving changed the frozen backdrop")
	}
}

func TestRenderSettingsThemeHasNoAbruptBrightnessStep(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(f func()) { done <- f }})
	v.Snapshot = s.Snapshot()
	v.Snapshot.Preferences.StaticBackground = true
	tt := ui.NewTester(v.Render, 800, 560)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	for _, label := range []string{"深色", "浅色"} {
		brightness := func() int { return int(tt.Image().RGBAAt(620, 420).G) }
		start := brightness()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		select {
		case apply := <-done:
			apply()
		case <-time.After(2 * time.Second):
			t.Fatal("theme was not saved")
		}
		prev, largest := start, 0
		for range 26 {
			tt.Advance(time.Second / 60)
			level := brightness()
			largest = max(largest, level-prev, prev-level)
			prev = level
		}
		span := max(start-prev, prev-start)
		if span < 60 || largest*7 > span {
			t.Errorf("%s: one frame changed by %d of a %d-level theme transition", label, largest, span)
		}
	}
}

func TestSettingsFlowKeepsLatestChoiceAndRestoresLastSaveOnError(t *testing.T) {
	repo := &fakeRepo{}
	s := service(t, &fakeEngine{}, repo)
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(f func()) { done <- f }})
	v.Snapshot = s.Snapshot()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	wait := func() {
		t.Helper()
		select {
		case apply := <-done:
			apply()
			tt.Frame()
		case <-time.After(2 * time.Second):
			t.Fatal("missing background save")
		}
	}
	click("设置")
	click("背景流光")
	click("背景流光")
	wait()
	wait()
	if s.Snapshot().Preferences.StaticBackground {
		t.Fatal("the second click did not restore motion")
	}
	repo.saveErr = errors.New("disk full")
	click("背景流光")
	wait()
	sw, _ := tt.AccessByRole(ui.RoleSwitch, "背景流光")
	if !sw.Checked || !tt.HasText("保存任务失败：disk full") {
		t.Fatal("failed save did not restore the switch and explain the error")
	}
}
