package presentation

import "github.com/egoist/mygo/ui"

// dropdown is the single trigger and menu surface for choices made in-place.
// MyGo owns placement, keyboard navigation, and checked items.
func dropdown(c *ui.Context, p palette, label, value string, disabled bool, build func(*ui.Menu)) ui.Element {
	trigger := ui.ButtonBase(c).Transition(controlMotion).Label(label).Disabled(disabled).
		FocusRing(true).Cursor(ui.CursorPointer).Row().Height(32).PaddingX(10).Gap(8).AlignItems(ui.Center).Radius(cornerSmall)
	open := ui.Local(trigger, "expanded", func() bool { return false })
	wasOpen := *open
	trigger.PopoverMenuButton(build, ui.PopoverMenuButtonOptions{
		Open: open,
		Panel: func(panel ui.Element) {
			panel.Padding(4).Radius(cornerField).Background(p.panel).Border(1, p.border).
				NoShadow().Shadow(0, 5, 16, 0, p.scrim.Alpha(.2)).Transition(dropdownMotion)
		},
		Item: func(item ui.Element) {
			item.Height(30).PaddingX(8).Radius(cornerSmall).Transition(controlMotion)
		},
	})
	if wasOpen && !*open && !disabled {
		trigger.Focus()
	}
	fill, edge := p.panel.Mix(p.tint, .22), p.border
	if trigger.Pressed() || *open {
		fill = p.panel.Mix(p.tint, .42)
	}
	if trigger.Focused() || *open {
		edge = p.active
	}
	angle := trigger.AnimateTo("chevron", 180*amount(*open), selectionSpring)
	trigger.Background(fill).Border(1, edge).Children(func() {
		ui.Text(c, value).Grow(1).MinWidth(0).SingleLine().TruncateMiddle().FontSize(13).ContentTransition(textChange)
		ui.Icon(c, icons["chevron"]).Size(14, 14).Shrink(0).Rotate(angle).TextColor(p.muted)
	})
	return trigger
}
