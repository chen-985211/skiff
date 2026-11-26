package transfer_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
	"skiff/internal/contexts/transfer/presentation/i18n"
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
			dir.Bounds.H > 95 || flow.Bounds.Y <= theme.Bounds.Y+theme.Bounds.H || dir.Bounds.Y < flow.Bounds.Y+flow.Bounds.H ||
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
	if !item.Checked || !tt.HasText("保存任务失败") {
		t.Fatal("failed save did not restore and explain the previous theme")
	}
	click("全部任务")
	if tt.HasText("保存任务失败") {
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
		// Sample the canvas and settings surface away from the sliding theme
		// selector. A pixel under its moving thumb measures a different motion.
		theme, ok := tt.AccessByRole(ui.RoleRadioGroup, "主题")
		dir, hasDir := tt.AccessByRole(ui.RoleButton, "更改默认保存目录")
		if !ok || !hasDir {
			t.Fatal("missing settings controls")
		}
		brightness := func() [2]int {
			im := tt.Image()
			y := int(theme.Bounds.Y + theme.Bounds.H/2)
			return [2]int{int(im.RGBAAt(int(dir.Bounds.X)-12, y).G), int(im.RGBAAt(int(theme.Bounds.X)-22, y).G)}
		}
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
		prev, largest := start, [2]int{}
		for range 26 {
			tt.Advance(time.Second / 60)
			level := brightness()
			for i := range level {
				largest[i] = max(largest[i], level[i]-prev[i], prev[i]-level[i])
			}
			prev = level
		}
		for i, area := range []string{"canvas", "settings surface"} {
			span := max(start[i]-prev[i], prev[i]-start[i])
			if span < 60 || largest[i]*7 > span {
				t.Errorf("%s %s: one frame changed by %d of a %d-level theme transition", label, area, largest[i], span)
			}
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
	if !sw.Checked || !tt.HasText("保存任务失败") {
		t.Fatal("failed save did not restore the switch and explain the error")
	}
}

func TestRenderConcurrencySaveOnlyChangesItsOwnControl(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { done <- fn }, PickDirectory: func(string, func(string, error)) {}})
	v.Snapshot = s.Snapshot()
	tt := interactionScene(v, 800, 560)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	apply := func() {
		t.Helper()
		select {
		case fn := <-done:
			fn()
			tt.Frame()
		case <-time.After(3 * time.Second):
			t.Fatal("no preference save")
		}
	}
	click("设置")
	dir, _ := tt.AccessByRole(ui.RoleButton, "更改默认保存目录")
	// A save must leave unrelated controls interactive and visually identical.
	sample := func() []byte {
		img := tt.Image()
		var pixels []byte
		r := dir.Bounds
		for y := int(r.Y) + 3; y < int(r.Y+r.H)-3; y += 2 {
			for x := int(r.X) + 3; x < int(r.X+r.W)-3; x += 2 {
				p := img.RGBAAt(x, y)
				pixels = append(pixels, p.R, p.G, p.B)
			}
		}
		return pixels
	}
	before := sample()
	click("同时下载 20 项")
	click("同时下载 30 项")
	other, _ := tt.AccessByRole(ui.RoleButton, "更改默认保存目录")
	if v.Busy || other.Disabled || !bytes.Equal(before, sample()) {
		t.Fatal("saving concurrency repainted or disabled the directory row")
	}
	apply()
	apply()
	if s.Snapshot().Preferences.Concurrency != 30 {
		t.Fatal("rapid selections did not save the final value")
	}
	field, ok := tt.AccessByRole(ui.RoleTextField, "自定义同时下载数")
	if !ok {
		t.Fatal("missing custom numeric field")
	}
	r := field.Bounds
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("201")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("请输入 1–200 的整数") || s.Snapshot().Preferences.Concurrency != 30 {
		t.Fatal("out-of-range input changed the queue", tt.Texts())
	}
	click("全部任务")
	click("设置")
	if tt.HasText("请输入 1–200 的整数") {
		t.Fatal("validation error remained after the invalid edit was discarded")
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("200")
	click("全部任务")
	apply()
	if s.Snapshot().Preferences.Concurrency != 200 {
		t.Fatal("custom limit was not saved")
	}
}

func TestRenderConcurrencySelectionTravelsBetweenPresets(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Preferences.Concurrency = 1
	tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	n, _ := tt.AccessByRole(ui.RoleButton, "同时下载 5 项")
	x, y := int(n.Bounds.X+n.Bounds.W/2), int(n.Bounds.Y+5)
	unchanged := tt.Image().RGBAAt(x, y)
	v.Snapshot.Preferences.Concurrency = 30
	tt.Frame()
	passed := false
	for range 35 {
		tt.Advance(8 * time.Millisecond)
		if tt.Image().RGBAAt(x, y) != unchanged {
			passed = true
		}
	}
	tt.Advance(time.Second)
	if !passed || tt.Image().RGBAAt(x, y) != unchanged {
		t.Fatal("selection did not travel across the intermediate preset and settle")
	}
}

func TestRenderConcurrencyHintStaysAlignedWhileChangingLanguage(t *testing.T) {
	locales := []struct{ tag, title, hint, preset, field string }{
		{"zh-CN", "同时下载数", "1–200 项，超出后排队", "同时下载 1 项", "自定义同时下载数"},
		{"en", "Concurrent downloads", "1–200 at once. The rest queue.", "Download 1 at a time", "Custom concurrent downloads"},
		{"zh-Hant", "同時下載數", "1–200 項，超過時排入佇列", "同時下載 1 項", "自訂同時下載數"},
		{"ja", "同時ダウンロード数", "1～200 件。超過分は待機", "同時に 1 件ダウンロード", "同時ダウンロード数を指定"},
	}
	aligned := func(a, b ui.Rect) bool {
		for _, delta := range []float32{a.X - b.X, a.Y - b.Y, a.W - b.W, a.H - b.H} {
			if math.Abs(float64(delta)) > .5 {
				return false
			}
		}
		return true
	}
	for _, size := range [][2]int{{800, 560}, {1020, 680}} {
		for _, reduced := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d/reduced_motion=%t", size[0], size[1], reduced), func(t *testing.T) {
				view := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
				view.Snapshot.Preferences.StaticBackground = true
				var hintWidth float32
				scene := ui.NewTesterWith(func(c *ui.Context) {
					hint := i18n.Format(c.Locale(), "settings.concurrencyHint", i18n.Args{"limit": application.MaxConcurrency})
					hintWidth, _ = c.MeasureText(0, ui.Span{Text: hint, Size: 12})
					view.Render(c)
				}, size[0], size[1], ui.TesterOptions{Preferences: ui.Preferences{
					ReduceMotion: reduced, ReduceTransparency: true,
				}})
				scene.SetLocale("zh-CN")
				chooseSource(t, scene, "设置")
				scene.Advance(time.Second)
				first := locales[0]
				initialTitle := sourceBounds(t, scene, ui.RoleText, first.title)
				initialHint := sourceBounds(t, scene, ui.RoleText, first.hint)
				initialPreset := sourceBounds(t, scene, ui.RoleButton, first.preset)
				initialField := sourceBounds(t, scene, ui.RoleTextField, first.field)
				pixels := func() []byte {
					img := scene.Image()
					var sample []byte
					for y := int(initialHint.Y); y < int(initialHint.Y+initialHint.H); y++ {
						for x := int(initialHint.X); x < int(initialPreset.X)-8; x++ {
							p := img.RGBAAt(x, y)
							sample = append(sample, p.R, p.G, p.B)
						}
					}
					return sample
				}
				for _, locale := range []int{1, 0, 2, 0, 3, 0} {
					current := locales[locale]
					before := pixels()
					scene.SetLocale(current.tag)
					scene.Advance(72 * time.Millisecond)
					middle := pixels()
					for _, remaining := range []time.Duration{0, time.Second} {
						scene.Advance(remaining)
						title := sourceBounds(t, scene, ui.RoleText, current.title)
						hint := sourceBounds(t, scene, ui.RoleText, current.hint)
						preset := sourceBounds(t, scene, ui.RoleButton, current.preset)
						field := sourceBounds(t, scene, ui.RoleTextField, current.field)
						if !aligned(title, initialTitle) || !aligned(hint, initialHint) ||
							!aligned(preset, initialPreset) || !aligned(field, initialField) ||
							field.X+field.W > float32(size[0]) || hint.X+hint.W > preset.X || hint.W < hintWidth || hint.H <= 0 {
							t.Errorf("%s moved or wrapped the subtitle or controls: title %v, hint %v, preset %v, field %v", current.tag, title, hint, preset, field)
						}
					}
					after := pixels()
					if bytes.Equal(before, after) || (!reduced && (bytes.Equal(before, middle) || bytes.Equal(middle, after))) {
						t.Errorf("%s subtitle has no visible in-place transition", current.tag)
					}
				}
			})
		}
	}
}
