package transfer_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

// Near the top edge of a 36-DIP corner, the continuous shoulder still curves
// past x=31. A circular arc already covers this point. Sample at 4× to see
// the difference independently of display scale and antialiasing.
func TestRenderWorkspaceContinuousCornerSilhouettes(t *testing.T) {
	for _, clip := range []bool{false, true} {
		scene := ui.NewTester(func(c *ui.Context) {
			white := ui.Hex("#ffffff")
			ui.Box(c).Fill().Background(ui.Hex("#000000")).Children(func() {
				surface := ui.Box(c).Size(160, 120).Radius(36)
				if clip {
					surface.Clip().Children(func() { ui.Box(c).Fill().Background(white) })
				} else {
					surface.Background(white)
				}
			})
		}, 160, 120)
		scene.SetScale(4)
		im := scene.Image()
		for _, x := range []int{124, 639 - 124} {
			for _, y := range []int{2, 479 - 2} {
				if got := im.RGBAAt(x, y).R; got > 72 {
					t.Errorf("clip=%t: circular shoulder at (%d,%d), got %d", clip, x, y, got)
				}
			}
		}
		if got := im.RGBAAt(320, 2).R; got < 240 {
			t.Errorf("clip=%t: straight edge is missing, got %d", clip, got)
		}
	}
}

func TestRenderTaskCardCornerSizes(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "one", Name: "file.zip", Status: "complete"}}
	scene := interactionScene(v, 1020, 680)
	for _, radius := range []float32{20, 22} {
		if radius == 22 {
			if err := scene.Click("查看详情 file.zip"); err != nil {
				t.Fatal(err)
			}
			scene.Advance(time.Second)
		}
		card, ok := scene.AccessByRole(ui.RoleGroup, "file.zip")
		if !ok {
			t.Fatal("task card missing")
		}
		// Compare the silhouette only. Using a reference surface keeps this
		// independent of the app palette and the renderer's antialiasing.
		reference := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.Hex("#000000")).Children(func() {
				ui.Box(c).Size(100, 100).Radius(radius).Background(ui.Hex("#ffffff"))
			})
		}, 100, 100).Image()
		frame := scene.Image()
		left, top := int(card.Bounds.X), int(card.Bounds.Y)
		canvas := frame.RGBAAt(left-1, top)
		for y := 0; y < 12; y++ {
			for x := 0; x < 32; x++ {
				coverage := reference.RGBAAt(x, y).R
				outside := frame.RGBAAt(left+x, top+y) == canvas
				if coverage == 0 && !outside || coverage == 255 && outside {
					t.Fatalf("radius %.0f: corner differs at (%d, %d)", radius, x, y)
				}
			}
		}
	}
}
