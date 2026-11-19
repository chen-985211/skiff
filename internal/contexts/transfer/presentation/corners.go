package presentation

import "github.com/egoist/mygo/ui"

// MyGo defaults to circular corners on Windows and Linux. Set the renderer
// policy before any window, tester or offscreen surface is created.
func init() { ui.SetContinuousCorners(true) }

// Radii are in device-independent points and shared by all renderers.
const (
	cornerProgress     = 2.5
	cornerInline       = 6
	cornerSmall        = 9
	cornerCompact      = 10
	cornerSettingsIcon = 11
	cornerField        = 12
	cornerRow          = 13
	cornerSegment      = 14
	cornerRemove       = 15
	cornerAction       = 16
	cornerSegmentTrack = 17
	cornerPrimary      = 18
	cornerCard         = 20
	cornerDetail       = 22
	cornerDock         = dockHeight / 2
	cornerSheet        = 22
)
