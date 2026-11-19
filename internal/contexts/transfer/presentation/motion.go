package presentation

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const fullProgressHold = 1200 * time.Millisecond

// Animated geometry uses springs: retargeting a live MyGo channel preserves
// both position and velocity. Progress and color have bounded endpoints.
var (
	themeFade       = ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.CubicBezier(.33, 0, .67, 1), NonMotion: true}
	pressSpring     = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 900, Damping: 60}}
	switchSpring    = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 625, Damping: 40}}
	selectionSpring = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 484, Damping: 44}}
	cardSpring      = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 235, Damping: 31}}
	panelSpring     = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 324, Damping: 36}}
	colorFade       = ui.Animation{Duration: 180 * time.Millisecond, Ease: ui.EaseInOut, Oklab: true}
	progressEase    = ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.EaseOut}
	completionFill  = ui.Animation{Duration: 800 * time.Millisecond, Ease: ui.EaseInOut}
	completionFade  = ui.Animation{Duration: 450 * time.Millisecond, Ease: ui.EaseInOut, Oklab: true}

	controlMotion = ui.ElementTransition{Animation: &pressSpring, Colors: true}
	pageFade      = ui.ElementTransition{Animation: &colorFade, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
	panelMotion   = ui.ElementTransition{Animation: &panelSpring, Enter: &ui.Motion{Y: 7}, Exit: &ui.Motion{Y: -5}}
	dockMotion    = ui.ElementTransition{Animation: &panelSpring, Enter: &ui.Motion{Y: 12}, Exit: &ui.Motion{Y: 12}}
	textChange    = ui.ContentTransition{Animation: &colorFade}
	numberChange  = ui.ContentTransition{Numeric: true, Animation: &colorFade}
)

// Keep the badge and card tint in step with the outgoing full progress bar.
func (f progressFrame) statusChange() ui.ContentTransition {
	if f.status == "complete" && f.opacity > 0 {
		return ui.ContentTransition{Animation: &completionFade}
	}
	return textChange
}

type cardMatch string

func cardMotion(id string) ui.ElementTransition {
	return ui.ElementTransition{Match: cardMatch(id), Fly: true, Animation: &cardSpring, Enter: &ui.Motion{Y: 9, Scale: .985}, Exit: &ui.Motion{Y: -5, Scale: .985}}
}

func amount(on bool) float32 {
	if on {
		return 1
	}
	return 0
}
