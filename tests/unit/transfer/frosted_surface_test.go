package transfer_test

import (
	"image/color"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation"
)

func TestWorkspaceFrostSheetHasNoDecorativeMotion(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	var backdrop ui.Rect
	scene := ui.NewTester(func(c *ui.Context) {
		v.Render(c)
		ui.Box(c).Absolute().Left(backdrop.X).Top(backdrop.Y).Width(backdrop.W).Height(backdrop.H).Background(ui.Hex("#aab2c0"))
	}, 800, 560)
	v.OpenNew()
	scene.Frame()
	scene.Advance(time.Second)
	sheet, ok := scene.AccessByRole(ui.RoleDialog, "")
	if !ok {
		t.Fatal("missing expanded sheet")
	}
	// Give the glass a uniform backdrop while the rest of the workspace animates.
	backdrop = sheet.Bounds
	backdrop.X, backdrop.Y = backdrop.X-60, backdrop.Y-60
	backdrop.W, backdrop.H = backdrop.W+120, backdrop.H+120
	scene.Frame()
	n, ok := scene.AccessByRole(ui.RoleTextField, "下载链接")
	if !ok {
		t.Fatal("missing expanded editor")
	}
	x1, x2 := int(n.Bounds.X)+30, int(n.Bounds.X+n.Bounds.W)-30
	strip := func(y int) []byte {
		im := scene.Image()
		return slices.Clone(im.Pix[im.PixOffset(x1, y):im.PixOffset(x2, y)])
	}
	y := int(n.Bounds.Y + n.Bounds.H/2)
	initial, workspace := strip(y), strip(84)
	if !slices.Equal(initial, strip(int(n.Bounds.Y)-5)) || !slices.Equal(initial, strip(int(n.Bounds.Y+n.Bounds.H)+5)) {
		t.Error("the header, editor and footer should share one glass surface")
	}
	scene.Type("https://example.org/file")
	scene.Advance(2 * time.Second)
	if !slices.Equal(initial, strip(y)) {
		t.Error("the form added a moving light to a still backdrop")
	}
	if slices.Equal(workspace, strip(84)) {
		t.Error("opening the sheet froze the workspace light")
	}
}

func TestWorkspaceOpeningSheetPreservesOutsideColor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		scene := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{
			Dark: dark, Preferences: ui.Preferences{ReduceMotion: true},
		})
		outside := func() [2]color.RGBA {
			im := scene.Image()
			return [2]color.RGBA{im.RGBAAt(80, 360), im.RGBAAt(740, 290)}
		}
		before := outside()
		v.OpenNew()
		scene.Frame()
		if _, ok := scene.AccessByRole(ui.RoleDialog, ""); !ok || outside() != before {
			t.Errorf("dark=%v: opening the sheet changed the canvas from %v to %v", dark, before, outside())
		}
	}
}

func TestWorkspaceFrostLinkFieldHasAnOutlineAndNoFill(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.OpenNew()
		scene := ui.NewTesterWith(v.Render, 1020, 680, ui.TesterOptions{
			Dark: dark, Preferences: ui.Preferences{ReduceMotion: true},
		})
		field, ok := scene.AccessByRole(ui.RoleTextField, "下载链接")
		if !ok {
			t.Fatal("missing link editor")
		}
		for _, focused := range []bool{true, false} {
			if !focused {
				if err := scene.Click("保存目录"); err != nil {
					t.Fatal(err)
				}
			}
			im := scene.Image()
			x, y := int(field.Bounds.X+field.Bounds.W/2), int(field.Bounds.Y)
			outside, edge, inside := im.RGBAAt(x, y-2).G, im.RGBAAt(x, y).G, im.RGBAAt(x, y+3).G
			if max(outside, edge)-min(outside, edge) < 8 || max(outside, inside)-min(outside, inside) > 5 {
				t.Errorf("dark=%v focused=%v: surrounding %d, outline %d, interior %d", dark, focused, outside, edge, inside)
			}
		}
	}
}

func TestWorkspaceFrostSubmitKeepsPrimaryButtonColor(t *testing.T) {
	for _, opts := range []ui.TesterOptions{
		{}, {Dark: true}, {Preferences: ui.Preferences{ReduceTransparency: true}},
		{Dark: true, Preferences: ui.Preferences{HighContrast: true}},
	} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		opts.Preferences.ReduceMotion = true
		scene := ui.NewTesterWith(v.Render, 1020, 680, opts)
		dock, ok := scene.AccessByRole(ui.RoleButton, "新建下载")
		if !ok {
			t.Fatal("missing primary button in dock")
		}
		accent := scene.Image().RGBAAt(int(dock.Bounds.X+dock.Bounds.W/2), int(dock.Bounds.Y)+6)
		v.OpenNew()
		for _, busy := range []bool{false, true} {
			v.Busy = busy
			scene.Frame()
			button, ok := scene.AccessByRole(ui.RoleButton, "开始下载")
			if !ok || button.Disabled != busy {
				t.Fatal("missing submit action or wrong enabled state")
			}
			r := button.Bounds
			x, y := int(r.X+r.W/2), int(r.Y)
			im := scene.Image()
			inside, outside := im.RGBAAt(x, y+5), im.RGBAAt(x, y-4)
			if !busy && inside != accent || busy && (inside == accent || inside == outside) {
				t.Errorf("dark=%v busy=%v: submit %v, primary %v, background %v", opts.Dark, busy, inside, accent, outside)
			}
		}
	}
}

func TestWorkspaceFrostDiffusesBackdropThroughoutSheet(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready, v.DialogOpen = true, true
	var band ui.Rect
	ink := ui.Hex("#000000")
	scene := ui.NewTesterWith(func(c *ui.Context) {
		v.Render(c)
		ui.Box(c).Absolute().Left(band.X).Top(band.Y).Width(band.W).Height(band.H).Background(ink)
	}, 1020, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	sheet, ok := scene.AccessByRole(ui.RoleDialog, "")
	if !ok {
		t.Fatal("missing expanded sheet")
	}
	r := sheet.Bounds
	// The band should show through the face, soften beyond its lower edge,
	// and reflect more strongly in the top rim than in the gap below the rim.
	band = ui.Rect{X: r.X, Y: r.Y + 28, W: r.W, H: 52}
	sample := func() (rim, gap, middle, below int) {
		scene.Frame()
		im := scene.Image()
		x, y := int(r.X+r.W*.76), int(r.Y)
		at := func(y int) int { return int(im.RGBAAt(x, y).G) }
		return at(y + 2), at(y + 14), at(y + 60), at(int(band.Y+band.H) + 12)
	}
	darkRim, darkGap, darkCenter, darkOutside := sample()
	ink = ui.Hex("#ffffff")
	lightRim, lightGap, lightCenter, lightOutside := sample()
	transmission := lightCenter - darkCenter
	if transmission < 55 || transmission > 190 {
		t.Errorf("expected the broad backdrop band to show through the interior; got %+d", transmission)
	}
	if lightOutside-darkOutside < transmission/7 {
		t.Errorf("backdrop band did not diffuse 12 points beyond its edge: %+d of %d", lightOutside-darkOutside, transmission)
	}
	if lightRim-darkRim < lightGap-darkGap+12 {
		t.Fatalf("backdrop change at the rim %+d, in the gap %+d", lightRim-darkRim, lightGap-darkGap)
	}
}

// Resize the static workspace to put different folds of the background behind
// each surface. Compare a strip free of text with the same strip uncovered.
func TestWorkspaceFrostedSurfacesTransmitBackground(t *testing.T) {
	for _, dark := range []bool{false, true} {
		opts := ui.TesterOptions{Dark: dark, Preferences: ui.Preferences{ReduceMotion: true}}
		for _, surface := range []string{"selection", "sheet"} {
			var covered, uncovered [2][]color.RGBA
			for i, width := range []int{800, 1600} {
				v := presentation.New(nil, presentation.Actions{})
				v.Ready, v.DialogOpen = true, surface == "sheet"
				scene := ui.NewTesterWith(v.Render, width, 560, opts)
				rect, ok := scene.Find("全部任务")
				if surface == "sheet" {
					panel, found := scene.AccessByRole(ui.RoleDialog, "")
					rect, ok = panel.Bounds, found
				}
				if !ok {
					t.Fatalf("missing %s", surface)
				}
				strip := func() []color.RGBA {
					im := scene.Image()
					var pixels []color.RGBA
					y := rect.Y + 12
					if surface == "sheet" {
						y = rect.Y + rect.H/2 // Inside the editor, clear of its text and outline.
					}
					for x := int(rect.X + rect.W*.58); x < int(rect.X+rect.W*.90); x += 3 {
						pixels = append(pixels, im.RGBAAt(x, int(y)))
					}
					return pixels
				}
				covered[i] = strip()
				v.Filter, v.DialogOpen = "active", false
				scene.Frame()
				uncovered[i] = strip()
			}
			change := func(pair [2][]color.RGBA) int {
				total := 0
				for i, a := range pair[0] {
					b := pair[1][i]
					total += int(max(a.R, b.R) - min(a.R, b.R))
					total += int(max(a.G, b.G) - min(a.G, b.G))
					total += int(max(a.B, b.B) - min(a.B, b.B))
				}
				return total
			}
			bare, frosted := change(uncovered), change(covered)
			if bare < len(uncovered[0])*12 || frosted < bare/4 {
				t.Errorf("%s dark=%v: glass transmitted %d of %d backdrop change", surface, dark, frosted, bare)
			}
		}
	}
}

// In either accessibility mode the selected row and the sheet should use the
// same opaque surface, even though they cover different parts of the window.
func TestWorkspaceFrostUsesSolidAccessibleSurfaces(t *testing.T) {
	for _, prefs := range []ui.Preferences{{ReduceTransparency: true}, {HighContrast: true}} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		scene := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: prefs})
		row, ok := scene.Find("全部任务")
		if !ok {
			t.Fatal("missing sidebar selection")
		}
		at := func(r ui.Rect, part float32) color.RGBA {
			return scene.Image().RGBAAt(int(r.X+r.W*part), int(r.Y+12))
		}
		solid := at(row, .72)
		if solid != at(row, .88) {
			t.Fatal("selection still translucent")
		}
		v.OpenNew()
		scene.Frame()
		scene.Advance(time.Second)
		sheet, ok := scene.AccessByRole(ui.RoleDialog, "")
		if !ok || at(sheet.Bounds, .60) != solid || at(sheet.Bounds, .86) != solid {
			t.Fatal("sheet should use the same solid fill as the sidebar selection")
		}
	}
}
