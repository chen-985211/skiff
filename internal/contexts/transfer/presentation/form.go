package presentation

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func (v *View) newDialog(c *ui.Context, p palette, anchor ui.Rect, reveal float32) {
	windowW, windowH := c.Size()
	const left = sidebarWidth + workspaceInset
	width := min(float32(420), windowW-left-workspaceInset)
	if anchor.W == 0 {
		// A menu or keyboard shortcut can open the form from Settings, without a dock.
		anchor = ui.Rect{X: left + min(cardWidth, width) - composeButtonWidth, Y: windowH - footerInset - dockHeight, W: composeButtonWidth, H: dockHeight}
	}
	closed := 1 - reveal
	shown := true // Keep the modal and its draft mounted until the spring reaches the button.
	ui.DialogBase(c, &shown, func(back, panel ui.Element) {
		back.Background(p.scrim.Alpha(.14 * reveal))
		panel.Absolute().Left(left + (anchor.X-left)*closed).Bottom(footerInset + (windowH-anchor.Y-anchor.H-footerInset)*closed).
			Width(anchor.W + (width-anchor.W)*reveal).Column().AlignItems(ui.Start).Clip()
		surface(c, panel, p, p.tint, 22, true).Shadow(0, 6*reveal, 20*reveal, 0, p.scrim.Alpha(.15*reveal))

		// The form is measured at its reading width even while only a sliver is visible.
		// Clipping its layout keeps text and inputs at their natural scale.
		buttonInk := max(0, 1-reveal/.35)
		ui.Box(c.Key("button-fill")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Radius(22).
			Background(p.accent.Alpha(buttonInk)).IgnoreInput(true)
		fields := min(max((reveal-.48)/.42, 0), 1)
		form := ui.Column(c.Key("form")).Width(width).Shrink(0).Padding(16, 20).Gap(12).
			Opacity(fields).IgnoreInput(!v.DialogOpen).AccessHidden(!v.DialogOpen)
		form.Children(func() {
			// The moving title occupies the leading side of this header.
			ui.Row(c).Height(26).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				count := 0
				for line := range strings.Lines(v.URLs) {
					if strings.TrimSpace(line) != "" {
						count++
					}
				}
				if count > 1 {
					ui.Textf(c, "已输入 %d 行", count).FontSize(11).TextColor(p.muted).ContentTransition(numberChange)
				}
			})
			v.composeFields(c, p)
		})
		if reveal < 1 {
			height := max(anchor.H, form.LayoutBounds().H)
			panel.Height(anchor.H + (height-anchor.H)*reveal)
		}

		// Keep the button's label visible throughout. Its plus gives way to the heading.
		title := ui.Row(c.Key("compose-title")).Absolute().Left(17 + 3*reveal).Top(9 + 7*reveal).
			Height(26).Gap(7 * closed).AlignItems(ui.Center).IgnoreInput(true)
		ink := c.Theme().Text.Mix(c.Theme().AccentText, buttonInk)
		title.Children(func() {
			ui.Icon(c, icons["plus"]).Width(15 * closed).Clip().FontSize(15).TextColor(ink.Alpha(closed))
			ui.Text(c, "新建下载").SingleLine().FontSize(13 + 6*reveal).FontWeight(600).TextColor(ink)
		})
		// Until the inputs are revealed, a second press belongs to the morphing
		// button. Keep keyboard focus in the editor so a paste is accepted immediately.
		if !v.Busy && (!v.DialogOpen || fields < .5) {
			hit := ui.Box(c.Key("reverse-compose")).Absolute().Left(0).Right(0).Top(0).Bottom(0).AccessHidden(true)
			if hit.Clicked() || !v.DialogOpen && back.OverlayShortcut(ui.Cmd, ui.KeyN) {
				if v.DialogOpen {
					v.DialogOpen = false
				} else {
					v.OpenNew()
				}
				c.Invalidate()
			}
		}
	})
	if !shown && !v.Busy {
		v.DialogOpen = false
		c.Invalidate()
	}
}

func (v *View) composeFields(c *ui.Context, p palette) {
	fill := p.panel
	if !c.Preferences().ReduceTransparency && !c.Preferences().HighContrast {
		fill = fill.Alpha(.66)
	}
	links := ui.TextAreaBase(c, &v.URLs).FillWidth().Lines(4, 7).Padding(12, 14).FontSize(13).FixedLineHeight(20).
		Radius(12).Background(fill).Label("下载链接").Placeholder("粘贴下载链接，每行一个").
		Description("支持 HTTP 和 HTTPS 链接").AutoFocus().Disabled(v.Busy)
	edge := p.border
	if links.Focused() {
		edge = p.active
	}
	if !c.Preferences().HighContrast {
		edge = edge.Alpha(.5)
	}
	links.Border(1, edge)
	if links.Changed() {
		v.FormError = ""
	}

	if v.FormError != "" {
		ui.Text(c, v.FormError).MaxHeight(68).Tooltip(v.FormError).FontSize(12).TextColor(c.Theme().Danger)
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		v.composeDestination(c, p)
		if primaryAction(c, "开始下载", "download", v.Busy).Width(106).Shrink(0).Clicked() {
			v.submit()
		}
	})
}

func (v *View) composeDestination(c *ui.Context, p palette) {
	ui.Row(c).Grow(1).MinWidth(0).Height(36).AlignItems(ui.Center).Gap(8).Children(func() {
		var label ui.Element
		if v.Actions.PickDirectory != nil {
			label = ui.ButtonBase(c.Key("pick-directory")).Label("选择保存目录").Tooltip("选择保存目录").
				Radius(6).FocusRing(true).Disabled(v.Busy)
		} else {
			label = ui.Row(c)
		}
		label.Row().Shrink(0).FillHeight().AlignItems(ui.Center).Gap(5).Children(func() {
			ui.Icon(c, icons["folder"]).FontSize(15).TextColor(p.muted)
			ui.Text(c, "保存到").FontSize(12).TextColor(p.muted)
			if v.Actions.PickDirectory != nil {
				ui.Icon(c, icons["chevron"]).FontSize(12).TextColor(p.muted)
			}
		})
		if v.Actions.PickDirectory != nil && label.Clicked() {
			v.Actions.PickDirectory(v.Directory, func(path string, err error) {
				if err != nil {
					v.FormError = err.Error()
				} else if path != "" {
					v.Directory, v.FormError = path, ""
				}
			})
		}
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Justify(ui.Center).Children(func() {
			dir := ui.TextInputBase(c, &v.Directory).Absolute().Left(0).Right(0).Top(0).Bottom(0).PaddingX(6).
				Radius(6).FontSize(12).Label("保存目录").Tooltip(v.Directory).Disabled(v.Busy)
			if dir.Focused() || c.Preferences().HighContrast {
				dir.Border(1, p.border)
			}
			if !dir.Focused() {
				dir.TextColor(ui.Transparent)
				ui.Text(c, v.Directory).Margin(0, 6).FontSize(12).SingleLine().TruncateMiddle().IgnoreInput(true).AccessHidden(true)
			}
			if dir.Changed() {
				v.FormError = ""
			}
		})
	})
}
