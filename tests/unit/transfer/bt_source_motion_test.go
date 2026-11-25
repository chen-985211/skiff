package transfer_test

import (
	"bytes"
	"image"
	"image/draw"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func sourceBounds(t *testing.T, tt *ui.Tester, role ui.Role, label string) ui.Rect {
	t.Helper()
	n, ok := tt.AccessByRole(role, label)
	if !ok {
		t.Fatalf("missing %s", label)
	}
	return n.Bounds
}

func chooseSource(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	if err := tt.Click(name); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
}

func renderedRegion(tt *ui.Tester, r image.Rectangle) []byte {
	out := image.NewRGBA(r)
	draw.Draw(out, r, tt.Image(), r.Min, draw.Src)
	return out.Pix
}

func TestRenderBTSourceSwitchSharesGridAndFadesContent(t *testing.T) {
	v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
	v.URLs = "https://example.test/archive.zip"
	v.OpenNew()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	link := sourceBounds(t, tt, ui.RoleTextField, "下载链接")
	rail := sourceBounds(t, tt, ui.RoleRadioGroup, "下载类型")
	submit := sourceBounds(t, tt, ui.RoleButton, "开始下载")
	// Inspect the actual glyphs inside the source field, away from the caret,
	// focus ring, tab indicator and footer, which animate independently.
	r := image.Rect(int(link.X)+25, int(link.Y)+9, int(link.X)+155, int(link.Y)+36)
	crop := func() []byte { return renderedRegion(tt, r) }
	before := crop()
	chooseSource(t, tt, "BT")
	if current := sourceBounds(t, tt, ui.RoleRadioGroup, "下载类型"); current != rail {
		t.Errorf("tab strip jumped on press: %v -> %v", rail, current)
	}
	if !bytes.Equal(before, crop()) {
		t.Error("the first BT frame discarded the visible link text")
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "下载链接"); ok {
		t.Error("outgoing source field still accepts input")
	}
	tt.Advance(60 * time.Millisecond)
	tt.Frame()
	mid := crop()
	chooseSource(t, tt, "链接")
	if !bytes.Equal(mid, crop()) {
		t.Error("reversal jumped away from the rendered frame")
	}
	tt.Advance(time.Second)
	if !bytes.Equal(before, crop()) {
		t.Error("link draft did not settle back to its original appearance")
	}
	chooseSource(t, tt, "BT")
	tt.Advance(125 * time.Millisecond)
	entering := crop()
	tt.Advance(time.Second)
	end := crop()
	if bytes.Equal(before, mid) || bytes.Equal(mid, end) || bytes.Equal(entering, end) || bytes.Equal(mid, entering) {
		t.Error("source content did not render distinct exit and entry frames")
	}
	if got := sourceBounds(t, tt, ui.RoleTextField, "磁力链接"); got.X != link.X || got.W != link.W || got.Y != link.Y {
		t.Errorf("source fields should share their top and side edges: %v -> %v", link, got)
	}
	if got := sourceBounds(t, tt, ui.RoleButton, "读取文件列表"); got != submit {
		t.Errorf("primary action moved: %v -> %v", submit, got)
	}
}

func TestRenderBTPreviewCaptionFitsOnOneLine(t *testing.T) {
	for _, tc := range []struct {
		width int
		theme application.ThemeMode
	}{{800, application.ThemeLight}, {1020, application.ThemeDark}} {
		v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
		v.Snapshot.Preferences.Theme = tc.theme
		v.Directory = strings.Repeat("/long-directory", 12)
		v.OpenNew()
		tt := interactionScene(v, tc.width, 560)
		chooseSource(t, tt, "BT")
		btn := sourceBounds(t, tt, ui.RoleButton, "读取文件列表")
		im := tt.Image()
		x, y := int(btn.X), int(btn.Y)
		fill := im.RGBAAt(x+7, y+int(btn.H)/2)
		first, last := int(btn.H), -1
		for dy := 1; dy < int(btn.H)-1; dy++ {
			for dx := 38; dx < int(btn.W)-16; dx++ {
				if math.Abs(float64(im.RGBAAt(x+dx, y+dy).R)-float64(fill.R)) > 80 {
					first, last = min(first, dy), max(last, dy)
					break
				}
			}
		}
		if last-first+1 < 8 || last-first+1 > 18 {
			t.Errorf("%s at %d: primary label spans rows %d..%d inside %v; want one readable line", tc.theme, tc.width, first, last, btn)
		}
	}
}

func TestRenderBTOptionsShowsItsFullCaption(t *testing.T) {
	for _, tc := range []struct{ locale, label, caption, field string }{
		{"zh-CN", "BT 选项", "BT 选项", "磁力链接"},
		{"en", "BitTorrent options", "Options", "Magnet link"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
			v.OpenNew()
			var captionWidth float32
			tt := ui.NewTesterWith(func(c *ui.Context) {
				captionWidth, _ = c.MeasureText(0, ui.Span{Text: tc.caption, Size: 13, Weight: 520})
				v.Render(c)
			}, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
			tt.SetLocale(tc.locale)
			chooseSource(t, tt, "BT")
			button := sourceBounds(t, tt, ui.RoleMenuButton, tc.label)
			field := sourceBounds(t, tt, ui.RoleTextField, tc.field)
			if !tt.HasText(tc.caption) {
				t.Errorf("expected visible caption %q in %v", tc.caption, tt.Texts())
			}
			// Reserve the rendered glyphs plus the dropdown's insets, gap and chevron.
			if room := button.W - 2*(10+1) - 8 - 14; room < float32(math.Ceil(float64(captionWidth))) {
				t.Errorf("options trigger leaves %.2f points for a %.2f-point label", room, captionWidth)
			}
			if button.X+button.W != field.X+field.W {
				t.Errorf("options trigger and editor do not share their trailing edge: %v, %v", button, field)
			}
		})
	}
}

func TestBTSourceSwitchRetargetsTallDraftAndRespectsReducedMotion(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
		v.URLs = strings.Repeat("https://example.test/file\n", 8)
		v.OpenNew()
		tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduced, ReduceTransparency: true}})
		tt.Advance(time.Second)
		height := func() float32 { return sourceBounds(t, tt, ui.RoleDialog, "新建下载").H }
		start := height()
		chooseSource(t, tt, "BT")
		first := height()
		tt.Advance(85 * time.Millisecond)
		tt.Frame()
		mid := height()
		chooseSource(t, tt, "链接")
		back := height()
		tt.Advance(time.Second)
		if height() != start {
			t.Fatal("wrapped draft lost its original height")
		}
		chooseSource(t, tt, "BT")
		tt.Advance(time.Second)
		end := height()
		if end >= start {
			t.Fatalf("tall link draft %.1f should collapse to the shorter BT page, got %.1f", start, end)
		}
		if reduced {
			if first != end || mid != end || back != start {
				t.Errorf("reduced motion did not reach the selected height immediately: %.1f, %.1f, %.1f", first, mid, back)
			}
		} else if first != start || mid <= end || mid >= start || back != mid {
			t.Errorf("reduced=%v: initial %.1f, press %.1f, moving %.1f, reverse %.1f, settled BT %.1f", reduced, start, first, mid, back, end)
		}
	}
}

func TestRenderBTReturnFromCreatePreservesFirstFrame(t *testing.T) {
	v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
	v.OpenNew()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	chooseSource(t, tt, "BT")
	tt.Advance(time.Second)
	chooseSource(t, tt, "BT 选项")
	chooseSource(t, tt, "创建种子…")
	tt.Advance(time.Second)
	source := sourceBounds(t, tt, ui.RoleTextField, "制种来源")
	x, y := int(source.X), int(source.Y)
	r := image.Rect(x+3, y+3, x+145, y+int(source.H)-3)
	before := renderedRegion(tt, r)
	tool := sourceBounds(t, tt, ui.RoleButton, "返回下载")
	xr, yt := int(tool.X+tool.W), int(tool.Y)
	chevron := image.Rect(xr-23, yt+12, xr-10, yt+20)
	chooseSource(t, tt, "返回下载")
	toolFirst := renderedRegion(tt, chevron)
	if !bytes.Equal(before, renderedRegion(tt, r)) {
		t.Error("returning to BT replaced the creator's visible glyphs in the first frame")
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "制种来源"); ok {
		t.Error("outgoing creator can still receive input")
	}
	tt.Advance(60 * time.Millisecond)
	outgoing := renderedRegion(tt, r)
	tt.Advance(75 * time.Millisecond)
	incoming := renderedRegion(tt, r)
	tt.Advance(time.Second)
	if bytes.Equal(before, outgoing) || bytes.Equal(outgoing, incoming) || bytes.Equal(incoming, renderedRegion(tt, r)) {
		t.Error("expected distinct exit and entry frames on return from the creator")
	}
	if bytes.Equal(toolFirst, renderedRegion(tt, chevron)) {
		t.Error("incoming BT options chevron appeared immediately on returning from create")
	}
}

func TestBTSourceKeyboardFocusFollowsSelection(t *testing.T) {
	v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	chooseSource(t, tt, "BT")
	for _, step := range []struct {
		key        ui.Key
		tab, field string
	}{{ui.KeyLeft, "链接", "下载链接"}, {ui.KeyRight, "BT", "磁力链接"}} {
		tt.Key(0, step.key)
		tt.Frame()
		if !tt.Focused(step.tab) {
			t.Errorf("keyboard focus did not follow selection to %s", step.tab)
		}
		sourceBounds(t, tt, ui.RoleTextField, step.field)
	}
}

func TestRenderEnglishPrimaryDownloadActionsFitWithoutMoving(t *testing.T) {
	verifyWidth := func(t *testing.T, scene *ui.Tester, label string, textWidth float32) ui.Rect {
		t.Helper()
		bounds := sourceBounds(t, scene, ui.RoleButton, label)
		if room := bounds.W - 2*12 - 7 - 15; room < float32(math.Ceil(float64(textWidth))) {
			t.Errorf("%q needs %.1f pt with %.1f pt available", label, textWidth, room)
		}
		return bounds
	}
	t.Run("getting engine", func(t *testing.T) {
		view := presentation.New(nil, presentation.Actions{DownloadEngine: func() {}})
		view.OfferEngineDownload(nil)
		var textWidth float32
		scene := ui.NewTesterWith(func(c *ui.Context) {
			textWidth, _ = c.MeasureText(0, ui.Span{Text: "Getting engine…", Size: 13, Weight: 600})
			view.Render(c)
		}, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
		scene.SetLocale("en")
		before := sourceBounds(t, scene, ui.RoleButton, "Get Engine")
		chooseSource(t, scene, "Get Engine")
		if after := verifyWidth(t, scene, "Getting engine…", textWidth); before != after {
			t.Errorf("engine action moved when its caption changed: %v to %v", before, after)
		}
	})
	t.Run("saving torrent", func(t *testing.T) {
		view := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
		view.OpenNew()
		var textWidth float32
		scene := ui.NewTesterWith(func(c *ui.Context) {
			textWidth, _ = c.MeasureText(0, ui.Span{Text: "Save Torrent…", Size: 13, Weight: 600})
			view.Render(c)
		}, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
		scene.SetLocale("en")
		chooseSource(t, scene, "BT")
		before := sourceBounds(t, scene, ui.RoleButton, "Fetch Files")
		chooseSource(t, scene, "BitTorrent options")
		chooseSource(t, scene, "Create Torrent…")
		if after := verifyWidth(t, scene, "Save Torrent…", textWidth); before.X != after.X || before.W != after.W {
			t.Errorf("composer action moved horizontally between editors: %v to %v", before, after)
		}
	})
}
