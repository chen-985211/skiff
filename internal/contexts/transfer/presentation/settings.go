package presentation

import (
	"context"

	"github.com/egoist/mygo/ui"
)

func (v *View) settingsPage(c *ui.Context, p palette) {
	ui.Column(c.Key("settings-page")).Grow(1).Gap(18).Padding(18, 4).Transition(panelMotion).Children(func() {
		ui.Text(c, "设置").FontSize(26).FontWeight(650)
		surface(c, ui.Column(c).Padding(20).Gap(20), p, p.tint, 18, false).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, "默认保存目录").FontSize(14).FontWeight(600)
					ui.Text(c, v.Snapshot.Directory).FontSize(12).TextColor(p.muted).Selectable()
				})
				if ui.Button(c, "选择文件夹…").Label("更改默认保存目录").Disabled(v.Busy || !v.Ready || v.Actions.PickDirectory == nil).Clicked() {
					v.Actions.PickDirectory(v.Snapshot.Directory, func(path string, err error) {
						if err != nil {
							v.Message = err.Error()
							return
						}
						if path != "" {
							v.run("", func(ctx context.Context) error { return v.API.SetDefaultDirectory(ctx, path) })
						}
					})
				}
			})
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				label := ui.Text(c, "背景流光").Grow(1).FontSize(14).FontWeight(600)
				enabled := !v.Snapshot.Preferences.StaticBackground && !c.Preferences().ReduceMotion
				toggle := ui.Switch(c, &enabled).LabelledBy(label).Disabled(v.Busy || !v.Ready || c.Preferences().ReduceMotion)
				if c.Preferences().ReduceMotion {
					toggle.Description("系统已启用减少动态效果")
				}
				if toggle.Changed() {
					still := !v.Snapshot.Preferences.StaticBackground
					v.run("", func(ctx context.Context) error { return v.API.SetStaticBackground(ctx, still) })
				}
			})
		})
	})
}
