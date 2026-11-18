package presentation

import (
	"context"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

func (v *View) settingsPage(c *ui.Context, p palette) {
	ui.Column(c.Key("settings-page")).Grow(1).MinHeight(0).Padding(32, 8).Transition(pageFade).Children(func() {
		list := ui.Column(c).MaxWidth(820).FillWidth().AlignSelf(ui.Center)
		list.Children(func() {
			ui.Text(c, "设置").FontSize(28).FontWeight(650).Margin(0, 0, 20)
			block := ui.Column(c).FillWidth().Radius(18)
			if c.Preferences().ReduceTransparency || c.Preferences().HighContrast {
				block.Background(p.panel)
			} else {
				block.LinearGradient(ui.LinearGradient{Angle: 117, Stops: []ui.GradientStop{
					{At: 0, Color: p.ice.Mix(p.panel, .67).Alpha(.64)},
					{At: .39, Color: p.panel.Alpha(.67)},
					{At: 1, Color: p.lilac.Mix(p.panel, .65).Alpha(.53)},
				}})
			}
			block.Children(func() {
				row := ui.ButtonBase(c.Key("directory-setting")).Label("更改默认保存目录").Description(v.Snapshot.Directory).Tooltip(v.Snapshot.Directory).Disabled(v.Busy || !v.Ready || v.Actions.PickDirectory == nil).Radius(16).Transition(controlMotion)
				if row.Hovered() {
					row.Background(p.panel.Alpha(.30))
				}
				settingsRow(c, row, "folder", "默认保存目录", v.directoryError, p, func(ui.Element) {
					ui.Text(c, v.Snapshot.Directory).Grow(1).MinWidth(0).SingleLine().TextAlign(ui.End).FontSize(13).TextColor(p.muted).ContentTransition(textChange)
					ui.Icon(c, icons["chevron"]).Rotate(-90).FontSize(14).TextColor(p.muted)
				})
				if row.Clicked() {
					v.pickDefaultDirectory()
				}
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "theme", "主题", v.themeSave.err, p, func(ui.Element) { v.themeSelector(c, p) })
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "light", "背景流光", v.flowSave.err, p, func(label ui.Element) { v.lightSwitch(c, p, label) })
			})
		})
	})
}

func settingsRow(c *ui.Context, row ui.Element, icon, name string, err error, p palette, control func(ui.Element)) {
	row.Row().FillWidth().Height(70).AlignItems(ui.Center).Justify(ui.Start).PaddingX(18).Gap(12).Children(func() {
		ui.Box(c).Size(32, 32).Center().Radius(11).Gradient(p.panel.Alpha(.74), p.tint.Alpha(.48), 135).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(18).TextColor(p.muted)
		})
		var label ui.Element
		ui.Column(c).MinWidth(0).Gap(3).Children(func() {
			label = ui.Text(c, name).FontSize(14).FontWeight(600).Shrink(0)
			inlineError(c, err)
		})
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Justify(ui.End).Gap(10).Children(func() {
			control(label)
		})
	})
}

func settingsDivider(c *ui.Context, p palette) {
	ui.Box(c).Height(1).Margin(0, 18, 0, 62).Background(p.border.Alpha(.46))
}

func (v *View) pickDefaultDirectory() {
	v.directoryError = nil
	v.Actions.PickDirectory(v.Snapshot.Directory, func(path string, err error) {
		if err != nil {
			v.directoryError = err
			return
		}
		if path != "" {
			v.run(func(ctx context.Context) error { return v.API.SetDefaultDirectory(ctx, path) }, func(err error) {
				v.directoryError = err
				if err == nil {
					v.openDirectoryError = nil
				}
			})
		}
	})
}

var themes = [...]struct {
	label string
	mode  application.ThemeMode
}{
	{"跟随系统", application.ThemeSystem}, {"浅色", application.ThemeLight}, {"深色", application.ThemeDark},
}

func (v *View) themeSelector(c *ui.Context, p palette) {
	index := 0
	for i, theme := range themes {
		if theme.mode == v.selectedTheme() {
			index = i
		}
	}
	parts := ui.SegmentedBase(c, &index, len(themes))
	const width = 64
	track := parts.Track.Label("主题").Height(34).Padding(3).Radius(17).Background(p.haze.Alpha(.26)).Disabled(!v.Ready || v.API == nil)
	x := track.AnimateTo("selected-theme", float32(index*width), selectionSpring)
	track.Children(func() {
		ui.Box(c).Absolute().Top(3).Left(3).Size(width, 28).Radius(14).Background(p.panel).Translate(x, 0).IgnoreInput(true).AccessHidden(true)
		for i, item := range themes {
			segment := parts.Segment(i).Size(width, 28).Radius(14).ZIndex(1).Label(item.label)
			segment.Children(func() { ui.Text(c, item.label).FontSize(12).FontWeight(570) })
			if segment.Changed() {
				v.themeSave.choose(v, &v.Snapshot.Preferences.Theme, item.mode, v.API.SetTheme)
				c.Invalidate()
			}
		}
	})
}

func (v *View) lightSwitch(c *ui.Context, p palette, label ui.Element) {
	reduced := c.Preferences().ReduceMotion
	enabled := !v.staticBackground() && !reduced
	toggle := ui.SwitchBase(c.Key("flow-switch"), &enabled).Size(48, 32).AlignItems(ui.Center).LabelledBy(label).Disabled(!v.Ready || v.API == nil || reduced)
	if reduced {
		toggle.Description("系统已启用减少动态效果")
	}
	position := toggle.AnimateTo("thumb", amount(enabled), switchSpring)
	track := p.border.Mix(p.muted, .30).Mix(p.active, max(float32(0), min(1, position)))
	toggle.Children(func() {
		ui.Box(c).Size(48, 26).Radius(13).Background(track).Children(func() {
			ui.Box(c).Absolute().Left(3).Top(3).Size(20, 20).Radius(10).Translate(22*position, 0).Background(p.panel).Shadow(0, 1, 2, 0, p.scrim.Alpha(.22))
		})
	})
	if toggle.Changed() {
		v.flowSave.choose(v, &v.Snapshot.Preferences.StaticBackground, !v.staticBackground(), v.API.SetStaticBackground)
		c.Invalidate()
	}
}
