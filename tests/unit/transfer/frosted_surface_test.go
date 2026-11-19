package transfer_test

import (
	"image/color"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation"
)

func TestWorkspaceFrostSheetHasOneToneOverMovingLight(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	scene := ui.NewTester(v.Render, 800, 560)
	scene.Advance(time.Second)
	v.OpenNew()
	scene.Frame()
	scene.Advance(time.Second)
	n, ok := scene.AccessByRole(ui.RoleTextField, "下载链接")
	if !ok {
		t.Fatal("missing expanded editor")
	}
	strip := func(x1, x2, y int) []byte {
		im := scene.Image()
		return slices.Clone(im.Pix[im.PixOffset(x1, y):im.PixOffset(x2, y)])
	}
	x1, x2 := int(n.Bounds.X)+14, int(n.Bounds.X+n.Bounds.W)-14
	y := int(n.Bounds.Y + n.Bounds.H/2)
	initial, workspace := strip(x1, x2, y), strip(240, 650, 84)
	// The lit face of the reference action sheet ranges from cool silver
	// to near-neutral pearl. A blue-grey or paper-white fill misses it.
	face := scene.Image().RGBAAt(x1, y)
	if face.R < 230 || face.R > 239 || face.G < face.R || face.B < face.G || face.B-face.R > 8 {
		t.Errorf("expected a light silver face, got %v", face)
	}
	titleGap := strip(x1, x2, int(n.Bounds.Y)-6)
	footerGap := strip(x1, x2, int(n.Bounds.Y+n.Bounds.H)+6)
	if !slices.Equal(initial, titleGap) || !slices.Equal(initial, footerGap) ||
		!slices.Equal(initial[:len(initial)-4], initial[4:]) {
		t.Error("the header, editor and footer should share one even frost")
	}
	scene.Type("https://example.org/file")
	scene.Advance(2 * time.Second)
	if !slices.Equal(initial, strip(x1, x2, y)) {
		t.Error("light moved across the open editor")
	}
	if slices.Equal(workspace, strip(240, 650, 84)) {
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

// Compare neighboring pixels on either side of the editor's top edge. Its
// focused border can stand out; the fill should stay close to the glass.
func TestWorkspaceFrostLinkFieldSharesSheetTone(t *testing.T) {
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
		im := scene.Image()
		luma := func(x, y int) int {
			pixel := im.RGBAAt(x, y)
			return (int(pixel.R)*21 + int(pixel.G)*72 + int(pixel.B)*7) / 100
		}
		for _, position := range []float32{.25, .50, .75} {
			x, y := int(field.Bounds.X+field.Bounds.W*position), int(field.Bounds.Y)
			diff := luma(x, y+4) - luma(x, y-4)
			if diff > 4 || diff < -20 {
				t.Errorf("dark=%v at %.0f%%: editor brightness relative to sheet = %+d", dark, 100*position, diff)
			}
		}
	}
}

func TestWorkspaceFrostSubmitSharesSheetTone(t *testing.T) {
	for _, opts := range []ui.TesterOptions{
		{}, {Dark: true}, {Preferences: ui.Preferences{ReduceTransparency: true}},
		{Dark: true, Preferences: ui.Preferences{HighContrast: true}},
	} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready, v.DialogOpen = true, true
		opts.Preferences.ReduceMotion = true
		scene := ui.NewTesterWith(v.Render, 1020, 680, opts)
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
			if inside != outside {
				t.Errorf("options=%+v busy=%v: submit fill %v breaks away from sheet %v", opts, busy, inside, outside)
			}
		}
	}
}

func TestWorkspaceFrostRefractsBackdropAtSheetEdge(t *testing.T) {
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
	// Place a band well inside the sheet. The curved top edge should pick up
	// its reflection, with an unaffected gap between the edge and the band.
	band = ui.Rect{X: r.X, Y: r.Y + 28, W: r.W, H: 52}
	sample := func() (int, int) {
		scene.Frame()
		im := scene.Image()
		x, y := int(r.X+r.W*.76), int(r.Y)
		return int(im.RGBAAt(x, y+2).G), int(im.RGBAAt(x, y+14).G)
	}
	darkRim, darkGap := sample()
	ink = ui.Hex("#ffffff")
	lightRim, lightGap := sample()
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
						y = rect.Y + 2 // Only the polished rim transmits the backdrop.
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
