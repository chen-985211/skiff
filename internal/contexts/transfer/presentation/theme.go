package presentation

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

type palette struct{ canvas, panel, border, muted, accent, green, tint, haze, warm, rim ui.Color }

func appearance(c *ui.Context) palette {
	t := *c.Theme()
	p := palette{
		haze: ui.Hex("#8a98b4"), warm: ui.Hex("#dcd5de"), rim: ui.Hex("#f8f9fa"), canvas: ui.Hex("#d6dae4"), panel: ui.Hex("#f8f9fa"),
		border: ui.Hex("#adb6c7"), muted: ui.Hex("#475366"), accent: ui.Hex("#202532"),
		green: ui.Hex("#24665d"), tint: ui.Hex("#c5cbd8"),
	}
	t.Text = ui.Hex("#1c2330")
	t.Danger, t.Warning = ui.Hex("#963f50"), ui.Hex("#78581d")
	if t.Dark {
		t.Danger, t.Warning = ui.Hex("#ffb5bc"), ui.Hex("#e9c780")
		p = palette{
			haze: ui.Hex("#495469"), warm: ui.Hex("#4b424e"), rim: ui.Hex("#8390a5"), canvas: ui.Hex("#292e39"), panel: ui.Hex("#363c48"),
			border: ui.Hex("#606a7a"), muted: ui.Hex("#c6cdd9"), accent: ui.Hex("#e6e7eb"),
			green: ui.Hex("#99d9c5"), tint: ui.Hex("#50596b"),
		}
		t.Text = ui.Hex("#f1f3f7")
	}
	if c.Preferences().HighContrast {
		p.muted = p.muted.Mix(t.Text, .45)
		p.border = p.border.Mix(t.Text, .45)
	}
	t = *t.WithAccent(p.accent)
	t.Background, t.Surface, t.Border, t.TextMuted = p.canvas, p.panel, p.border, p.muted
	t.SurfaceHover, t.SurfacePressed = p.tint, p.tint.Mix(p.accent, .12)
	t.Success, t.Focus, t.Selection = p.green, p.muted, p.haze.Alpha(.40)
	t.Radius = 9
	c.SetTheme(&t)
	return p
}

// The dock and sheet sample the surface beneath them. Cards use a quieter
// translucent fill so that scrolling a large grid needs no per-card blur.
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
