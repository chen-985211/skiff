package presentation

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

type palette struct {
	shade                                                                                                    float32
	canvas, panel, border, muted, accent, complete, paused, tint, haze, warm, rim, ice, lilac, active, scrim ui.Color
}

// One mix value drives the workspace, shader and controls, including when
// switching back to the system appearance halfway through a transition.
func appearance(c *ui.Context, shade float32) palette {
	t := *c.Theme()
	if dark := shade >= .5; dark != t.Dark {
		base := ui.LightTheme()
		if dark {
			base = ui.DarkTheme()
		}
		t = *base.WithPreferences(c.Preferences())
	}
	pair := func(day, night string) ui.Color { return ui.Hex(day).Mix(ui.Hex(night), shade) }
	p := palette{
		shade:  shade,
		canvas: pair("#d6dae4", "#292e39"), panel: pair("#f8f9fa", "#363c48"), rim: pair("#f8f9fa", "#8390a5"),
		haze: pair("#8a98b4", "#495469"), warm: pair("#dcd5de", "#4b424e"), tint: pair("#c5cbd8", "#50596b"),
		ice: pair("#b9e5e7", "#739ca9"), lilac: pair("#c5b8d7", "#9586a9"),
		border: pair("#adb6c7", "#606a7a"), muted: pair("#475366", "#c6cdd9"), accent: pair("#202532", "#e6e7eb"),
		complete: pair("#565e92", "#c1c6f1"), paused: pair("#665f73", "#cbc1d4"), active: pair("#326889", "#9cd4ed"), scrim: pair("#303443", "#131822"),
	}
	t.Text = pair("#1c2330", "#f1f3f7")
	t.Danger, t.Warning = pair("#963f50", "#ffb5bc"), pair("#78581d", "#e9c780")
	if c.Preferences().HighContrast {
		p.muted = p.muted.Mix(t.Text, .45)
		p.border = p.border.Mix(t.Text, .45)
	}
	t = *t.WithAccent(p.accent)
	t.AccentText = pair("#ffffff", "#18181b")
	t.Inverse, t.InverseText = t.Text, p.panel
	t.Background, t.Surface, t.Border, t.TextMuted = p.canvas, p.panel, p.border, p.muted
	t.SurfaceHover, t.SurfacePressed = p.tint, p.tint.Mix(p.accent, .12)
	t.Success, t.Focus, t.Selection = p.complete, p.muted, p.haze.Alpha(.40)
	t.Radius = 9
	c.SetTheme(&t)
	return p
}

// The dock and sheet frost their backdrop; navigation uses a translucent fill.
func surface(c *ui.Context, e ui.Element, p palette, tint ui.Color, radius float32, frosted bool) ui.Element {
	e.Radius(radius)
	if c.Preferences().ReduceTransparency || c.Preferences().HighContrast {
		return e.Background(p.panel).Border(1, p.border)
	}
	if frosted {
		return e.Material(glass.Glass{Style: glass.Regular, Tint: p.panel.Alpha(.43)})
	}
	return e.Gradient(p.panel.Alpha(.97), tint.Mix(p.panel, .56).Alpha(.82), 136)
}

func iconButton(c *ui.Context, name, label string, disabled bool) bool {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(label)).Size(30, 30).Center().Radius(9).Transition(controlMotion).Label(label).Tooltip(label).Disabled(disabled)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, icons[name]).FontSize(16).TextColor(t.TextMuted) })
	return b.Clicked()
}

func primaryAction(c *ui.Context, title, icon string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(title).Disabled(disabled).Height(36).PaddingX(12).Radius(18).Transition(controlMotion)
	fill := t.Accent
	if b.Pressed() {
		fill = t.AccentPressed
	} else if b.Hovered() {
		fill = t.AccentHover
	}
	b.Background(fill).Children(func() {
		ui.Row(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(7).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(15).TextColor(t.AccentText)
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(t.AccentText)
		})
	})
	return b
}

func secondaryAction(c *ui.Context, title string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(title)).Label(title).Height(32).PaddingX(14).Radius(16).Center().Transition(controlMotion)
	fill := t.Surface
	if b.Hovered() {
		fill = t.SurfaceHover
	}
	if b.Pressed() {
		fill = t.SurfacePressed
	}
	b.Background(fill).Children(func() { ui.Text(c, title).FontSize(12).FontWeight(560) })
	return b
}
