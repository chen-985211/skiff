package transfer_test

import (
	"testing"

	"github.com/egoist/mygo/ui"
	_ "skiff/internal/contexts/transfer/presentation" // Apply Skiff's renderer policy.
)

// Near the top edge of a 36-DIP corner, the continuous shoulder still curves
// past x=31. A circular arc already covers this point. Sample at 4× to see
// the difference independently of display scale and antialiasing.
func TestWorkspaceContinuousCornerSilhouettes(t *testing.T) {
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
