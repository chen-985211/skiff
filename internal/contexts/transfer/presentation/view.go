package presentation

import (
	"context"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

type API interface {
	SetTheme(context.Context, application.ThemeMode) error
	SetDefaultDirectory(context.Context, string) error
	SetStaticBackground(context.Context, bool) error
	Add(context.Context, string, string) (int, error)
	Execute(context.Context, string, application.Command) error
	SetPaused(context.Context, []string, bool) error
	Snapshot() application.Snapshot
}
type Actions struct {
	OpenDirectory func(string)
	Dispatch      func(func())
	PickDirectory func(string, func(string, error))
	Reveal        func(string)
}
type View struct {
	themeSave                                          preferenceSave[application.ThemeMode]
	flowSave                                           preferenceSave[bool]
	lastDefault                                        string
	settingsOpen                                       bool
	light                                              windowLight
	API                                                API
	Actions                                            Actions
	Snapshot                                           application.Snapshot
	Ready, Busy, DialogOpen                            bool
	RestartOpen                                        bool
	RestartID                                          string
	Filter, Query, URLs, Directory, FormError, Message string
	detailID                                           string
	grid                                               ui.GridState
}

func New(api API, actions Actions) *View {
	return &View{API: api, Actions: actions, Filter: "all", Ready: api != nil}
}
func (v *View) OpenNew() {
	if v.Ready && !v.Busy && !v.DialogOpen && !v.RestartOpen {
		v.DialogOpen = true
		if v.URLs == "" && (v.Directory == "" || v.Directory == v.lastDefault) {
			v.Directory = v.Snapshot.Directory
			v.lastDefault = v.Directory
		}
		v.FormError = ""
	}
}
func (v *View) Render(c *ui.Context) {
	stage := ui.Row(c).Fill().AlignItems(ui.Stretch)
	mode := v.selectedTheme()
	dark := mode == application.ThemeDark || mode == application.ThemeSystem && c.Theme().Dark
	shade := stage.AnimateTo("theme", amount(dark), themeFade)
	if c.Preferences().HighContrast {
		shade = amount(dark)
	}
	p := appearance(c, shade)
	stage.Background(p.canvas).TextColor(c.Theme().Text)
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		v.OpenNew()
	}
	translucent := !c.Preferences().ReduceTransparency && !c.Preferences().HighContrast
	if translucent {
		still := c.Preferences().ReduceMotion || v.staticBackground()
		stage.Draw(func(painter *ui.Painter, r ui.Rect) { v.light.draw(painter, r, still, shade) })
	} else {
		v.light.moving = false
	}
	stage.Children(func() {
		v.sidebar(c, p)
		main := ui.Column(c).Grow(1).FillHeight().MinWidth(0).Padding(0, 24)
		main.Children(func() {
			ui.Box(c).Height(max(20, c.TitleBar().Bottom)).FillWidth().DragWindow()
			if v.Message != "" {
				ui.Row(c.Key("message")).Transition(panelMotion).Padding(9, 12).Margin(0, 0, 10).Gap(8).AlignItems(ui.Center).Background(c.Theme().Warning.Alpha(.09)).Radius(9).Children(func() {
					ui.Icon(c, icons["error"]).FontSize(16).TextColor(c.Theme().Warning)
					ui.Text(c, v.Message).ContentTransition(textChange).FontSize(12).Grow(1).MinWidth(0).MaxHeight(64).Tooltip(v.Message)
					if v.Ready && iconButton(c, "close", "关闭提示", false) {
						v.Message = ""
					}
				})
			}
			if v.settingsOpen {
				v.settingsPage(c, p)
			} else {
				v.queue(c, p)
				v.dock(c, p)
			}
		})
	})
	// Keep the animation on the workspace, which is already on screen
	// before the sheet is built and survives dismissal or a quick reopen.
	target := float32(0)
	if v.DialogOpen {
		target = 1
	}
	reveal := stage.AnimateTo("compose", target, panelSpring)
	v.newDialog(c, p, reveal)
	if ui.AlertDialog(c, &v.RestartOpen, "从头重新下载？", "原来的未完成文件会保留，新下载使用不同文件名。", "取消", "重新下载") == 1 {
		v.command(v.RestartID, application.Restart)
	}
}
