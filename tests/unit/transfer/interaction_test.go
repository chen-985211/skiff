package transfer_test

import (
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

// Layout and input tests use the real widget tree with static, opaque surfaces.
// Motion and material tests select their own preferences.
func interactionScene(v *presentation.View, width, height int) *ui.Tester {
	return ui.NewTesterWith(v.Render, width, height, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: true, ReduceTransparency: true},
	})
}
