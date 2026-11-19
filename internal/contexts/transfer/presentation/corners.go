package presentation

import "github.com/egoist/mygo/ui"

// MyGo defaults to circular corners on Windows and Linux. Set the renderer
// policy before any window, tester or offscreen surface is created.
func init() { ui.SetContinuousCorners(true) }

// Continuous shoulders extend further along a straight edge than a circular
// radius. Leave room for that transition, particularly on short controls.
const (
	cornerProgress = 1.75
	cornerInline   = 5
	cornerSmall    = 8
	cornerField    = 10
	cornerRow      = 11
	cornerAction   = 13
	cornerPrimary  = 14
	cornerDock     = 17
	cornerCard     = 18
	cornerDetail   = 20
	cornerSheet    = 21
)
