package presentation

import (
	"context"
	"skiff/internal/contexts/transfer/presentation/i18n"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

// Keep the draggable track outside the aligned settings surfaces.
const settingsScrollGutter = 16

func settingsScrollbar(c *ui.Context, p palette) ui.ScrollbarStyle {
	thumb := p.muted.Alpha(.40)
	if c.Preferences().HighContrast {
		thumb = p.muted
	}
	return ui.ScrollbarStyle{
		TrackWidth: 12, ThumbWidth: 4, Thumb: thumb, Hover: p.muted.Alpha(.78), Active: p.muted,
	}
}

func (v *View) settingsPage(c *ui.Context, p palette) {
	ui.Scroll(c.Key("settings-page")).ScrollbarStyle(settingsScrollbar(c, p)).Grow(1).MinHeight(0).Transition(pageFade).Children(func() {
		list := ui.Column(c).MaxWidth(820).FillWidth().AlignSelf(ui.Center).Padding(12, settingsScrollGutter, 12, 8)
		list.Children(func() {
			// Fallback fonts can exceed the line height, so reserve the heading boxes in layout.
			const titleHeight, sectionHeight = 42, 18
			ui.Text(c, tr(c, "common.settings")).SingleLine().FontSize(28).FontWeight(650).
				Height(titleHeight).FixedLineHeight(titleHeight).Margin(0, 0, 4)
			section := func(label string, body func()) {
				ui.Text(c, label).SingleLine().Margin(9, 3, 5).FontSize(12).FontWeight(610).
					Height(sectionHeight).FixedLineHeight(sectionHeight).TextColor(p.muted)
				block := ui.Column(c).FillWidth().Radius(cornerPrimary)
				if c.Preferences().ReduceTransparency || c.Preferences().HighContrast {
					block.Background(p.panel)
				} else {
					block.LinearGradient(ui.LinearGradient{Angle: 117, Stops: []ui.GradientStop{{At: 0, Color: p.ice.Mix(p.panel, .67).Alpha(.64)}, {At: .39, Color: p.panel.Alpha(.67)}, {At: 1, Color: p.lilac.Mix(p.panel, .65).Alpha(.53)}}})
				}
				block.Children(body)
			}

			section(tr(c, "settings.appearance"), func() {
				settingsRow(c, ui.Row(c), "theme", tr(c, "settings.theme"), "", v.themeSave.err, p, func(ui.Element) { v.themeSelector(c, p) })
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "light", tr(c, "settings.background"), "", v.flowSave.err, p, func(label ui.Element) { v.lightSwitch(c, p, label) })
				if v.Actions.ChosenLocale != nil && v.Actions.SetLocale != nil {
					settingsDivider(c, p)
					settingsRow(c, ui.Row(c), "network", tr(c, "language.title"), "", nil, p, func(ui.Element) { v.languageSelector(c, p) })
				}
			})
			section(tr(c, "settings.downloads"), func() {
				row := ui.ButtonBase(c.Key("directory-setting")).Label(tr(c, "settings.changeDefaultFolder")).Description(v.Snapshot.Directory).Tooltip(v.Snapshot.Directory).Disabled(v.Busy || !v.Ready || v.Actions.PickDirectory == nil).Radius(cornerAction).Transition(controlMotion)
				if row.Hovered() {
					row.Background(p.panel.Alpha(.30))
				}
				settingsRow(c, row, "folder", tr(c, "settings.defaultFolder"), "", v.directoryError, p, func(ui.Element) {
					ui.Text(c, v.Snapshot.Directory).Grow(1).MinWidth(0).SingleLine().TruncateMiddle().TextAlign(ui.End).FontSize(13).TextColor(p.muted).ContentTransition(textChange)
					ui.Icon(c, icons["chevron"]).Rotate(-90).FontSize(14).TextColor(p.muted)
				})
				if row.Clicked() {
					v.pickDefaultDirectory()
				}
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "download", tr(c, "settings.concurrency"), trf(c, "settings.concurrencyHint", i18n.Args{"limit": application.MaxConcurrency}), v.concurrencySave.err, p, func(ui.Element) { v.concurrencySetting(c, p) })
				settingsDivider(c, p)
				settingsRow(c, ui.Row(c), "clock", tr(c, "settings.globalLimit"), "", v.globalRateErr, p, func(ui.Element) { v.speedSelector(c, p, "", v.Snapshot.Preferences.DownloadLimit) })
				settingsDivider(c, p)
				disk := tr(c, "common.auto")
				if v.Snapshot.Preferences.Disk != (application.DiskSettings{}) {
					disk = tr(c, "common.custom")
				}
				if v.settingsLink(c, p, "tray", "disk.title", disk) {
					v.openDisk(v.Snapshot.Preferences.Disk)
				}
			})
			section(tr(c, "settings.network"), func() {
				network := v.Snapshot.Preferences.Network
				proxy := tr(c, "network.systemProxy")
				switch network.Mode {
				case "direct":
					proxy = tr(c, "network.direct")
				case "manual":
					proxy = tr(c, "network.manual")
				}
				if v.settingsLink(c, p, "network", "network.proxyDNS", proxy) {
					v.openNetwork()
				}
				if _, ok := v.API.(btNetworkAPI); ok {
					settingsDivider(c, p)
					if v.settingsLink(c, p, "network", "bt.network", tr(c, "bt.networkHint")) {
						v.openBTNetwork()
					}
				}
			})
		})
	})
}

func (v *View) settingsLink(c *ui.Context, p palette, icon, labelKey, value string) bool {
	label := tr(c, labelKey)
	row := ui.ButtonBase(c.Key(labelKey)).Label(label).Description(value).Disabled(!v.Ready || v.Busy).Radius(cornerAction).FocusRing(true).Transition(controlMotion)
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
		labels := ui.Column(c).MinWidth(0).Gap(3)
		if hint != "" {
			labels.Grow(1).ClipX()
		}
		labels.Children(func() {
			const titleHeight, hintHeight = 22, 18
			label = ui.Text(c, name).FontSize(14).FontWeight(600).Shrink(0)
			if hint != "" {
				label.SingleLine().Height(titleHeight).FixedLineHeight(titleHeight)
			}
			if err == nil && hint != "" {
				ui.Text(c.Key("setting-hint"), hint).FillWidth().SingleLine().Tooltip(hint).
					Transition(pageFade).ContentTransition(textChange).FontSize(12).
					Height(hintHeight).FixedLineHeight(hintHeight).TextColor(p.muted)
			} else {
				inlineError(c, err)
			}
		})
		controls := ui.Row(c).MinWidth(0).AlignItems(ui.Center).Justify(ui.End).Gap(10)
		if hint != "" {
			controls.Shrink(0)
		} else {
			controls.Grow(1)
		}
		controls.Children(func() {
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
	{"common.system", application.ThemeSystem}, {"settings.light", application.ThemeLight}, {"settings.dark", application.ThemeDark},
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
	track := parts.Track.Label(tr(c, "settings.theme")).Height(34).Padding(3).Radius(cornerSegmentTrack).Background(p.haze.Alpha(.26)).Disabled(!v.Ready || v.API == nil)
	x := track.AnimateTo("selected-theme", float32(index*width), selectionSpring)
	track.Children(func() {
		ui.Box(c).Absolute().Top(3).Left(3).Size(width, 28).Radius(cornerSegment).Background(p.panel).Translate(x, 0).IgnoreInput(true).AccessHidden(true)
		for i, item := range themes {
			segment := parts.Segment(i).Size(width, 28).Radius(cornerSegment).ZIndex(1).Label(tr(c, item.label))
			segment.Children(func() { ui.Text(c, tr(c, item.label)).FontSize(12).FontWeight(570) })
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
		toggle.Description(tr(c, "settings.reduceMotion"))
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
	track := ui.Row(c.Key("concurrency-presets")).Label(tr(c, "settings.concurrencyPresets")).Padding(2).Radius(cornerCompact).Background(p.haze.Alpha(.21))
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
			preset := ui.ButtonBase(c.Key(strconv.Itoa(n))).Size(width, 28).ZIndex(1).Transition(controlMotion).Center().Radius(cornerSmall).Label(trf(c, "settings.concurrencyPreset", i18n.Args{"count": n})).AccessSelected(limit == n).Disabled(disabled)
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
	field := ui.TextInputBase(c.Key("concurrency-number"), &v.concurrencyText).Transition(controlMotion).Size(62, 32).PaddingX(8).FontSize(13).FontFeatures("tnum").TextAlign(ui.Center).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.50)).Label(tr(c, "settings.customConcurrency")).Description(tr(c, "settings.customConcurrencyHint")).Tooltip(tr(c, "settings.customConcurrencyLabel")).Disabled(disabled).FocusRing(true)
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
			v.concurrencySave.err = copyError("settings.concurrencyError", i18n.Args{"limit": application.MaxConcurrency})
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
