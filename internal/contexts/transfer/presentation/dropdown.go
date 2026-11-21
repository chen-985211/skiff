package presentation

import "github.com/egoist/mygo/ui"

// dropdown owns the trigger and the in-window menu. Its local accent colors
// are used by MyGo for both the highlighted label and its check mark.
func dropdown(c *ui.Context, p palette, label, value string, disabled bool, build func(*ui.Menu), icon ...*ui.SVG) ui.Element {
	prefs := c.Preferences()
	menu := *c.Theme()
	menu.Surface = p.panel.Mix(p.tint, .16)
	menu.Accent = p.tint.Mix(p.active, .16)
	menu.AccentText = menu.Text
	menu.Border = menu.Surface.Mix(p.border, .48)
	if prefs.HighContrast {
		menu.Border = p.border
	}
	return ui.Themed(c, &menu).Background(ui.Transparent).AlignItems(ui.Stretch).Children(func() {
		trigger := ui.ButtonBase(c).Transition(controlMotion).Label(label).Disabled(disabled).FocusRing(true).Cursor(ui.CursorPointer).
			Row().Height(32).PaddingX(10).Gap(8).AlignItems(ui.Center).Radius(cornerAction)
		open := ui.Local(trigger, "expanded", func() bool { return false })
		wasOpen := *open
		var menuPanel ui.Element
		trigger.PopoverMenuButton(build, ui.PopoverMenuButtonOptions{
			Open: open,
			Panel: func(panel ui.Element) {
				menuPanel = panel
				panel.AttachedOrigin().MinWidth(max(144, trigger.Bounds().W)).Padding(4).Radius(cornerField).Background(menu.Surface).Border(1, menu.Border).
					NoShadow().Shadow(0, 5, 16, 0, p.scrim.Alpha(.20)).Transition(dropdownMotion)
			},
			Item: func(item ui.Element) {
				item.Height(30).PaddingX(8).Radius(cornerSmall).FontSize(13)
				if item.Highlighted() && !item.IsDisabled() {
					dropdownHighlight(menuPanel, item, menu.Accent)
				}
			},
		})
		if wasOpen && !*open && !disabled {
			trigger.Focus()
		}
		fill, edge := p.panel.Mix(p.tint, .35), ui.Transparent
		if !prefs.ReduceTransparency && !prefs.HighContrast {
			fill = fill.Alpha(.55)
		}
		if prefs.HighContrast {
			edge = p.border
		}
		if trigger.Hovered() {
			fill = p.panel.Mix(p.tint, .8)
		}
		if trigger.Pressed() || *open {
			fill = menu.Accent
		}
		if trigger.FocusVisible() {
			edge = p.active
		}
		angle := trigger.AnimateTo("chevron", 180*amount(*open), selectionSpring)
		trigger.Background(fill).Border(1, edge).Children(func() {
			if len(icon) > 0 {
				ui.Icon(c, icon[0]).Size(15, 15).TextColor(p.muted).AccessHidden(true)
			}
			if value != "" {
				ui.Text(c, value).Transition(pageFade).Grow(1).MinWidth(0).SingleLine().TruncateMiddle().FontSize(13).FontWeight(520).ContentTransition(textChange)
			}
			ui.Icon(c, icons["chevron"]).Size(14, 14).Shrink(0).Rotate(angle).TextColor(p.muted)
		})
	})
}

// Paint the moving highlight behind every row. Text, check marks and input
// targets stay in their layout positions even when a key reverses the motion.
func dropdownHighlight(panel, item ui.Element, color ui.Color) {
	row, box := item.LayoutBounds(), panel.LayoutBounds()
	if row.H == 0 || box.H == 0 {
		return // The first layout still has MyGo's row highlight.
	}
	item.Background(ui.Transparent)
	left := row.X - box.X
	top := panel.AnimateTo("highlight-y", row.Y-box.Y, selectionSpring)
	panel.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Clip(r, cornerField, func() {
			p.Fill(ui.Rect{X: r.X + left, Y: r.Y + top, W: row.W, H: row.H}, color, cornerSmall)
		})
	})
}
