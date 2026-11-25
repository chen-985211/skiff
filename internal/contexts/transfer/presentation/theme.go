package presentation

import (
	"math"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

type palette struct {
	sheetFace                                                                                                         ui.Color
	shade                                                                                                             float32
	canvas, panel, border, muted, accent, complete, paused, waiting, tint, haze, warm, rim, ice, lilac, active, scrim ui.Color
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
		sheetFace: pair("#e8eaee", "#454a54"),
		shade:     shade,
		canvas:    pair("#d6dae4", "#292e39"), panel: pair("#f8f9fa", "#363c48"), rim: pair("#f8f9fa", "#8390a5"),
		haze: pair("#8a98b4", "#495469"), warm: pair("#dcd5de", "#4b424e"), tint: pair("#c5cbd8", "#50596b"),
		ice: pair("#b9e5e7", "#739ca9"), lilac: pair("#c5b8d7", "#9586a9"),
		border: pair("#adb6c7", "#606a7a"), muted: pair("#475366", "#c6cdd9"), accent: pair("#202532", "#e6e7eb"),
		complete: pair("#46586f", "#cbbfff"), paused: pair("#656477", "#d0c5d5"), active: pair("#365879", "#a2ceff"), waiting: pair("#627188", "#bcc9da"), scrim: pair("#303443", "#131822"),
	}
	t.Text = pair("#1c2330", "#f1f3f7")
	t.Danger, t.Warning = pair("#90465d", "#ffb5bc"), pair("#78581d", "#e9c780")
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
	t.Radius = cornerSmall
	c.SetTheme(&t)
	return p
}

type glassFinish uint8

const (
	searchGlass glassFinish = iota
	selectionFrost
	sheetFrost
)

func surface(c *ui.Context, e ui.Element, p palette, radius float32, finish glassFinish) ui.Element {
	e.Radius(radius)
	if c.Preferences().ReduceTransparency || c.Preferences().HighContrast {
		return e.Background(p.panel).Border(1, p.border)
	}
	if finish == searchGlass {
		return e.Material(glass.Glass{Style: glass.Regular, Tint: p.panel.Alpha(.43)})
	}

	if finish == sheetFrost {
		// Diffuse the backdrop before refracting it, keeping the silver tint light.
		return e.Material(glass.Blur{Radius: 20}).Draw(func(painter *ui.Painter, r ui.Rect) {
			glass.Paint(painter, r, radius, glass.Glass{Style: glass.Clear, Tint: p.sheetFace.Alpha(.28)})
		})
	}

	wash := ui.LinearGradient{Angle: 115, Stops: []ui.GradientStop{
		{At: 0, Color: p.tint.Mix(p.rim, .36).Alpha(.50)},
		{At: .45, Color: p.ice.Mix(p.panel, .86).Alpha(.35)},
		{At: 1, Color: p.rim.Alpha(.50)},
	}}
	return e.Material(glass.Blur{Radius: 7}).Draw(func(painter *ui.Painter, r ui.Rect) {
		painter.FillGradient(r, wash, radius)
	}).InsetShadow(0, -1, 2, 0, p.haze.Alpha(.32)).Border(1, ui.Transparent).BorderLinearGradient(ui.LinearGradient{
		Angle: 150, From: p.rim.Alpha(.82), To: p.haze.Alpha(.38),
	})
}

func iconButton(c *ui.Context, name, label string, disabled bool) bool {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(label)).Size(30, 30).Center().Radius(cornerSmall).Transition(controlMotion).Label(label).Tooltip(label).Disabled(disabled)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, icons[name]).FontSize(16).TextColor(t.TextMuted) })
	return b.Clicked()
}

const (
	primaryActionInset      = 12
	primaryActionGap        = 7
	primaryActionIconSize   = 15
	primaryActionFontSize   = 13
	primaryActionFontWeight = 600
)

func primaryActionWidth(c *ui.Context, minimum float32, captions ...string) float32 {
	for _, caption := range captions {
		textWidth, _ := c.MeasureText(0, ui.Span{Text: caption, Size: primaryActionFontSize, Weight: primaryActionFontWeight})
		minimum = max(minimum, float32(math.Ceil(float64(textWidth)))+2*primaryActionInset+primaryActionGap+primaryActionIconSize)
	}
	return minimum
}

func primaryAction(c *ui.Context, title, icon string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(title).Disabled(disabled).Height(36).PaddingX(primaryActionInset).Radius(cornerPrimary).Transition(controlMotion)
	fill := t.Accent
	if b.Pressed() {
		fill = t.AccentPressed
	} else if b.Hovered() {
		fill = t.AccentHover
	}
	return b.Background(fill).Children(func() {
		primaryActionContent(c, ui.Row(c).Fill(), title, icon)
	})
}

func primaryActionContent(c *ui.Context, row ui.Element, title, icon string) {
	ink := c.Theme().AccentText
	row.AlignItems(ui.Center).Justify(ui.Center).Gap(primaryActionGap).Children(func() {
		ui.Icon(c, icons[icon]).FontSize(primaryActionIconSize).TextColor(ink)
		ui.Text(c, title).SingleLine().FontSize(primaryActionFontSize).FontWeight(primaryActionFontWeight).TextColor(ink).ContentTransition(textChange)
	})
}

func secondaryAction(c *ui.Context, title string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(title)).Label(title).Height(32).PaddingX(14).Radius(cornerAction).Center().Transition(controlMotion)
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
