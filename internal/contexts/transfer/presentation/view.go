package presentation

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

type API interface {
	Add(context.Context, string, string) (int, error)
	Execute(context.Context, string, application.Command) error
	SetPaused(context.Context, []string, bool) error
	Snapshot() application.Snapshot
}
type Actions struct {
	Dispatch      func(func())
	PickDirectory func(string, func(string, error))
	Reveal        func(string)
}
type View struct {
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
		if v.Directory == "" && v.URLs == "" {
			v.Directory = v.Snapshot.Directory
		}
		v.FormError = ""
	}
}
func (v *View) Render(c *ui.Context) {
	p := appearance(c)
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		v.OpenNew()
	}
	stage := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(p.canvas)
	translucent := !c.Preferences().ReduceTransparency && !c.Preferences().HighContrast
	if translucent {
		stage.LinearGradient(ui.LinearGradient{Angle: 123, Stops: []ui.GradientStop{
			{Color: p.haze, At: 0}, {Color: p.canvas, At: .28}, {Color: p.panel, At: .45},
			{Color: p.haze.Mix(p.canvas, .22), At: .58}, {Color: p.tint, At: .73},
			{Color: p.warm, At: .87}, {Color: p.canvas, At: 1},
		}})
	}
	stage.Children(func() {
		v.sidebar(c, p)
		main := ui.Column(c).Grow(1).FillHeight().MinWidth(0).Padding(0, 24)
		if translucent {
			main.RadialGradient(ui.RadialGradient{Center: ui.Point{X: .87, Y: .89}, Stops: []ui.GradientStop{
				{Color: p.panel.Alpha(.48), At: 0}, {Color: p.warm.Alpha(.52), At: .14},
				{Color: p.haze.Alpha(.32), At: .28}, {Color: p.haze.Alpha(0), At: .54},
			}})
		}
		main.Children(func() {
			ui.Box(c).Height(max(20, c.TitleBar().Bottom)).FillWidth().DragWindow()
			if v.Message != "" {
				ui.Row(c).Padding(9, 12).Margin(0, 0, 10).Gap(8).AlignItems(ui.Center).Background(c.Theme().Warning.Alpha(.09)).Radius(9).Children(func() {
					ui.Icon(c, icons["error"]).FontSize(16).TextColor(c.Theme().Warning)
					ui.Text(c, v.Message).FontSize(12).Grow(1).MinWidth(0).MaxHeight(64).Tooltip(v.Message)
					if v.Ready && iconButton(c, "close", "关闭提示", false) {
						v.Message = ""
					}
				})
			}
			v.queue(c, p)
			v.dock(c, p)
		})
	})
	// Keep the animation on the workspace, which is already on screen
	// before the sheet is built and survives dismissal or a quick reopen.
	target := float32(0)
	if v.DialogOpen {
		target = 1
	}
	reveal := stage.Animate("compose", target, 270*time.Millisecond)
	v.newDialog(c, p, reveal)
	if ui.AlertDialog(c, &v.RestartOpen, "从头重新下载？", "原来的未完成文件会保留，新下载使用不同文件名。", "取消", "重新下载") == 1 {
		v.command(v.RestartID, application.Restart)
	}
}
