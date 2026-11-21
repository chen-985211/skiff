package presentation

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

// Keep the draggable track outside the aligned settings surfaces.
const settingsScrollGutter = 16

func settingsScroll(c *ui.Context, p palette) ui.Element {
	thumb := p.muted.Alpha(.40)
	if c.Preferences().HighContrast {
		thumb = p.muted
	}
	return ui.Scroll(c).ScrollbarStyle(ui.ScrollbarStyle{
		TrackWidth: 12, ThumbWidth: 4, Thumb: thumb, Hover: p.muted.Alpha(.78), Active: p.muted,
	})
}

func (v *View) settingsPage(c *ui.Context, p palette) {
	settingsScroll(c.Key("settings-page"), p).Grow(1).MinHeight(0).Transition(pageFade).Children(func() {
		list := ui.Column(c).MaxWidth(820).FillWidth().AlignSelf(ui.Center).Padding(12, settingsScrollGutter, 12, 8)
		list.Children(func() {
			ui.Text(c, "设置").FontSize(28).FontWeight(650).Margin(0, 0, 4)
			section := func(label string, body func()) {
				ui.Text(c, label).Margin(9, 3, 5).FontSize(12).FontWeight(610).TextColor(p.muted)
				block := ui.Column(c).FillWidth().Radius(cornerPrimary)
				if c.Preferences().ReduceTransparency || c.Preferences().HighContrast {
					block.Background(p.panel)
				} else {
					block.LinearGradient(ui.LinearGradient{Angle: 117, Stops: []ui.GradientStop{{At: 0, Color: p.ice.Mix(p.panel, .67).Alpha(.64)}, {At: .39, Color: p.panel.Alpha(.67)}, {At: 1, Color: p.lilac.Mix(p.panel, .65).Alpha(.53)}}})
				}
				block.Children(body)
			}

			section("下载", func() {
				row := ui.ButtonBase(c.Key("directory-setting")).Label("更改默认保存目录").Description(v.Snapshot.Directory).Tooltip(v.Snapshot.Directory).Disabled(v.Busy || !v.Ready || v.Actions.PickDirectory == nil).Radius(cornerAction).Transition(controlMotion)
				if row.Hovered() {
					row.Background(p.panel.Alpha(.30))
				}
				settingsRow(c, row, "folder", "默认保存目录", "", v.directoryError, p, func(ui.Element) {
					ui.Text(c, v.Snapshot.Directory).Grow(1).MinWidth(0).SingleLine().TruncateMiddle().TextAlign(ui.End).FontSize(13).TextColor(p.muted).ContentTransition(textChange)
					ui.Icon(c, icons["chevron"]).Rotate(-90).FontSize(14).TextColor(p.muted)
				})
				if row.Clicked() {
					v.pickDefaultDirectory()
				}
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "download", "同时下载数", fmt.Sprintf("1–%d 项，超出后排队", application.MaxConcurrency), v.concurrencySave.err, p, func(ui.Element) { v.concurrencySetting(c, p) })
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "clock", "全局限速", "", nil, p, func(ui.Element) { v.speedSelector(c, p, "", v.Snapshot.Preferences.DownloadLimit) })
				settingsDivider(c, p)
				disk := "自动"
				if v.Snapshot.Preferences.Disk != (application.DiskSettings{}) {
					disk = "自定义"
				}
				if v.settingsLink(c, p, "tray", "性能与磁盘", disk) {
					v.openDisk(v.Snapshot.Preferences.Disk)
				}
			})
			section("网络", func() {
				network := v.Snapshot.Preferences.Network
				proxy := "系统代理"
				switch network.Mode {
				case "direct":
					proxy = "直连"
				case "manual":
					proxy = "手动代理"
				}
				if v.settingsLink(c, p, "network", "代理与 DNS", proxy) {
					v.openNetwork()
				}
				if _, ok := v.API.(btNetworkAPI); ok {
					settingsDivider(c, p)
					if v.settingsLink(c, p, "network", "BT 网络", "节点、Tracker 与做种") {
						v.openBTNetwork()
					}
				}
			})
			section("外观", func() {
				settingsRow(c, ui.Row(c), "theme", "主题", "", v.themeSave.err, p, func(ui.Element) { v.themeSelector(c, p) })
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "light", "背景流光", "", v.flowSave.err, p, func(label ui.Element) { v.lightSwitch(c, p, label) })
			})
		})
	})
}

func (v *View) settingsLink(c *ui.Context, p palette, icon, label, value string) bool {
	row := ui.ButtonBase(c.Key(label)).Label(label).Description(value).Disabled(!v.Ready || v.Busy).Radius(cornerAction).FocusRing(true).Transition(controlMotion)
	if row.Hovered() {
		row.Background(p.panel.Alpha(.30))
	}
	settingsRow(c, row, icon, label, "", nil, p, func(ui.Element) {
		ui.Text(c, value).FontSize(13).TextColor(p.muted).ContentTransition(textChange)
		ui.Icon(c, icons["chevron"]).Rotate(-90).FontSize(14).TextColor(p.muted)
	})
	return row.Clicked()
}
func settingsRow(c *ui.Context, row ui.Element, icon, name, hint string, err error, p palette, control func(ui.Element)) {
	_, h := c.Size()
	row.Row().FillWidth().Height(min(60, max(44, (h-218)/8))).AlignItems(ui.Center).Justify(ui.Start).PaddingX(18).Gap(12).Children(func() {
		ui.Box(c).Size(32, 32).Center().Radius(cornerSettingsIcon).Gradient(p.panel.Alpha(.74), p.tint.Alpha(.48), 135).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(18).TextColor(p.muted)
		})
		var label ui.Element
		ui.Column(c).MinWidth(0).Gap(3).Children(func() {
			label = ui.Text(c, name).FontSize(14).FontWeight(600).Shrink(0)
			if err == nil && hint != "" {
				ui.Text(c.Key("setting-hint"), hint).Transition(pageFade).FontSize(12).TextColor(p.muted)
			} else {
				inlineError(c, err)
			}
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
	track := parts.Track.Label("主题").Height(34).Padding(3).Radius(cornerSegmentTrack).Background(p.haze.Alpha(.26)).Disabled(!v.Ready || v.API == nil)
	x := track.AnimateTo("selected-theme", float32(index*width), selectionSpring)
	track.Children(func() {
		ui.Box(c).Absolute().Top(3).Left(3).Size(width, 28).Radius(cornerSegment).Background(p.panel).Translate(x, 0).IgnoreInput(true).AccessHidden(true)
		for i, item := range themes {
			segment := parts.Segment(i).Size(width, 28).Radius(cornerSegment).ZIndex(1).Label(item.label)
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
	toggle := ui.SwitchBase(c.Key("flow-switch"), &enabled).Transition(controlMotion).Size(48, 32).AlignItems(ui.Center).LabelledBy(label).Disabled(!v.Ready || v.API == nil || reduced)
	if reduced {
		toggle.Description("系统已启用减少动态效果")
	}
	position := toggle.AnimateTo("thumb", amount(enabled), switchSpring)
	track := p.border.Mix(p.muted, .30).Mix(p.active, max(float32(0), min(1, position)))
	toggle.Children(func() {
		ui.Box(c).Size(48, 26).Radius(cornerRow).Background(track).Children(func() {
			ui.Box(c).Absolute().Left(3).Top(3).Size(20, 20).Radius(cornerCompact).Translate(22*position, 0).Background(p.panel).Shadow(0, 1, 2, 0, p.scrim.Alpha(.22))
		})
	})
	if toggle.Changed() {
		v.flowSave.choose(v, &v.Snapshot.Preferences.StaticBackground, !v.staticBackground(), v.API.SetStaticBackground)
		c.Invalidate()
	}
}

func (v *View) concurrencyLimit() int {
	limit := v.concurrencySave.current(v.Snapshot.Preferences.Concurrency)
	if limit == 0 {
		return application.DefaultConcurrency
	}
	return limit
}

func (v *View) concurrencySetting(c *ui.Context, p palette) {
	limit := v.concurrencyLimit()
	disabled := !v.Ready || v.API == nil
	presets := [...]int{1, 3, 5, 10, 20, 30}
	const width = 30
	track := ui.Row(c.Key("concurrency-presets")).Label("常用同时下载数").Padding(2).Radius(cornerCompact).Background(p.haze.Alpha(.21))
	// Keep the last position when a custom value fades the selection out.
	last := ui.Local(track, "last-preset", func() int { return 0 })
	selected := false
	for i, n := range presets {
		if n == limit {
			*last = i
			selected = true
			break
		}
	}
	x := track.AnimateTo("selection-x", float32(*last*width), selectionSpring)
	alpha := track.AnimateTo("selection-opacity", amount(selected), colorFade)
	track.Children(func() {
		ui.Box(c.Key("selection")).Absolute().Left(2).Top(2).Size(width, 28).Radius(cornerSmall).Background(p.panel).Translate(x, 0).Opacity(alpha).IgnoreInput(true).AccessHidden(true)
		for _, n := range presets {
			preset := ui.ButtonBase(c.Key(strconv.Itoa(n))).Size(width, 28).ZIndex(1).Transition(controlMotion).Center().Radius(cornerSmall).Label(fmt.Sprintf("同时下载 %d 项", n)).AccessSelected(limit == n).Disabled(disabled)
			if preset.Pressed() {
				preset.Background(p.panel.Alpha(.66))
			} else if preset.Hovered() {
				preset.Background(p.panel.Alpha(.48))
			}
			preset.Children(func() { ui.Textf(c, "%d", n).FontSize(12).FontFeatures("tnum").FontWeight(570) })
			if preset.Clicked() {
				v.concurrencyEdited = false
				v.saveConcurrency(n)
				c.Invalidate()
			}
		}
	})
	field := ui.TextInputBase(c.Key("concurrency-number"), &v.concurrencyText).Transition(controlMotion).Size(62, 32).PaddingX(8).FontSize(13).FontFeatures("tnum").TextAlign(ui.Center).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.50)).Label("自定义同时下载数").Description("1–200，回车或离开输入框时保存").Tooltip("自定义同时下载数（1–200）").Disabled(disabled).FocusRing(true)
	if field.Focused() {
		field.Border(1, p.active)
	}
	if field.Changed() {
		v.concurrencyEdited = true
		v.concurrencySave.err = nil
	}
	if v.concurrencyEdited && (field.Submitted() || !field.Focused()) {
		v.commitConcurrency(field.Focused())
	}
	if !v.concurrencyEdited {
		text := strconv.Itoa(v.concurrencyLimit())
		if v.concurrencyText != text {
			v.concurrencyText = text
			c.Invalidate()
		}
	}
}

func (v *View) commitConcurrency(keepInvalidDraft bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v.concurrencyText))
	if err != nil || n < 1 || n > application.MaxConcurrency {
		v.concurrencyEdited = keepInvalidDraft
		v.concurrencySave.err = nil
		if keepInvalidDraft {
			v.concurrencySave.err = fmt.Errorf("请输入 1–%d 的整数", application.MaxConcurrency)
		}
		return
	}
	v.concurrencyEdited = false
	v.saveConcurrency(n)
}

func (v *View) saveConcurrency(limit int) {
	v.concurrencySave.err = nil
	v.concurrencySave.choose(v, &v.Snapshot.Preferences.Concurrency, limit, v.API.SetConcurrency)
}
