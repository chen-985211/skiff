package transfer_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation"
)

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
					for x := int(rect.X + rect.W*.58); x < int(rect.X+rect.W*.90); x += 3 {
						pixels = append(pixels, im.RGBAAt(x, int(rect.Y+12)))
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
