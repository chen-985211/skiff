package presentation

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Animated geometry uses springs: retargeting a live MyGo channel preserves
// both position and velocity. Progress and color have bounded endpoints.
var (
	themeFade       = ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.CubicBezier(.33, 0, .67, 1), NonMotion: true}
	pressSpring     = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 900, Damping: 60}}
	switchSpring    = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 625, Damping: 40}}
	selectionSpring = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 484, Damping: 44}}
	panelSpring     = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 324, Damping: 36}}
	colorFade       = ui.Animation{Duration: 180 * time.Millisecond, Ease: ui.EaseInOut, Oklab: true}
	progressEase    = ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.EaseOut}

	controlMotion = ui.ElementTransition{Animation: &pressSpring, Colors: true}
	pageFade      = ui.ElementTransition{Animation: &colorFade, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
	panelMotion   = ui.ElementTransition{Animation: &panelSpring, Enter: &ui.Motion{Y: 7}, Exit: &ui.Motion{Y: -5}}
	dockMotion    = ui.ElementTransition{Animation: &panelSpring, Enter: &ui.Motion{Y: 12}, Exit: &ui.Motion{Y: 12}}
	scrimMotion   = ui.ElementTransition{Animation: &colorFade, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
	textChange    = ui.ContentTransition{Animation: &colorFade}
	numberChange  = ui.ContentTransition{Numeric: true, Animation: &colorFade}
)

type cardMatch string

func cardMotion(id string) ui.ElementTransition {
	return ui.ElementTransition{Match: cardMatch(id), Fly: true, ScaleContent: true, Animation: &panelSpring, Enter: &ui.Motion{Y: 9, Scale: .985}, Exit: &ui.Motion{Y: -5, Scale: .985}}
}

func amount(on bool) float32 {
	if on {
		return 1
	}
	return 0
}
