package presentation

import "github.com/egoist/mygo/ui"

const (
	fileTab = iota
	headerTab
	proxyTab
)

type optionTabItem struct{ name, caption, icon string }

var optionTabs = [...]optionTabItem{
	{"download.fileAndConnections", "common.file", "file"},
	{"headers.title", "headers.title", "headers"},
	{"network.proxy", "network.proxy", "network"},
}

const (
	optionLabelWidth = 82
	optionGap        = 8
	optionGutter     = 8 // the scroll thumb lives in the sheet margin
)

func (v *View) optionToolbar(c *ui.Context, p palette, width, ink float32) {
	optionTabBar(c, p, tr(c, "download.advancedTitle"), &v.optionTab, optionTabs[:], width).
		Opacity(editorOpacity(ink)).IgnoreInput(!v.advanced).AccessHidden(!v.advanced).Disabled(v.Busy)
}

func optionTabBar(c *ui.Context, p palette, label string, selected *int, choices []optionTabItem, width float32) ui.Element {
	const inset = 3
	cell := (width - 2*inset) / float32(len(choices))
	tabs := ui.TabsBase(c, selected, len(choices))
	rail := tabs.List.Label(label).Width(width).Height(formHeaderHeight).Padding(inset).
		Radius(cornerCompact).Background(p.haze.Alpha(.25))
	x := rail.AnimateTo("active-tab", float32(*selected)*cell, selectionSpring)
	rail.Draw(func(painter *ui.Painter, r ui.Rect) {
		painter.Fill(ui.Rect{X: r.X + inset + x, Y: r.Y + inset, W: cell, H: r.H - 2*inset}, p.panel, cornerSmall)
	})
	rail.Children(func() {
		for i, option := range choices {
			tab := tabs.Tab(i).ZIndex(1).Transition(controlMotion).Label(tr(c, option.name)).Tooltip(tr(c, option.name)).Width(cell).FillHeight().
				Center().Gap(5).Radius(cornerSmall).FocusRing(true).FocusRingOffset(-2)
			tint := tab.AnimateTo("selection-ink", amount(i == *selected), colorFade)
			color := p.muted.Mix(p.active, tint)
			if tab.Hovered() {
				color = c.Theme().Text
			}
			tab.Children(func() {
				ui.Icon(c, icons[option.icon]).FontSize(15).TextColor(color).AccessHidden(true)
				ui.Text(c, tr(c, option.caption)).SingleLine().FontSize(12).FontWeight(600).TextColor(color)
			})
		}
	})
	return rail
}

func (v *View) advancedOptions(c *ui.Context, p palette, width, height, reveal float32) {
	viewport := ui.Box(c.Key("option-pages")).Absolute().Top(0).Left(0).
		Width(width + optionGutter).Height(height).Clip().Opacity(editorOpacity(reveal)).
		AccessHidden(!v.advanced).IgnoreInput(!v.advanced).Disabled(v.Busy)
	viewport.Children(func() {
		for i, option := range optionTabs {
			active := i == v.optionTab
			weight := viewport.AnimateTo(i, amount(active), colorFade)
			if !active && weight == 0 {
				continue
			}
			scroll := ui.Scroll(c.Key(option.name)).Absolute().Top(0).Left(0).
				Width(width + optionGutter).Height(height).Clip().Opacity(editorOpacity(weight)).IgnoreInput(!active).AccessHidden(!active)
			scroll.Children(func() {
				ui.Column(c).Width(width).Gap(12).Padding(2, 0, 3).Children(func() {
					switch i {
					case fileTab:
						v.fileOptions(c, p)
					case headerTab:
						v.headerOptions(c, p)
					case proxyTab:
						v.proxyOptions(c, p, &v.taskNetwork, true, width)
					}
				})
			})
		}
	})
}

func formRow(c *ui.Context, p palette, label string, control func()) {
	ui.Row(c.Key(label)).FillWidth().Height(32).Shrink(0).Gap(optionGap).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).Width(optionLabelWidth).FontSize(12).FontWeight(560).TextColor(p.muted).SingleLine()
		control()
	})
}
func rowInput(c *ui.Context, p palette, label, placeholder string, value *string, secret bool) ui.Element {
	field := ui.TextInputBase(c.Key(label), value).Transition(controlMotion).Grow(1).MinWidth(0).Height(32).PaddingX(9).FontSize(13).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.42)).FocusRing(true).Label(label).Placeholder(placeholder)
	if field.Focused() {
		field.Border(1, p.active)
	}
	if secret {
		field.Password()
	}
	return field
}
func (v *View) fileOptions(c *ui.Context, p palette) {
	formRow(c, p, tr(c, "download.fileName"), func() {
		rowInput(c, p, tr(c, "download.saveSingleAs"), tr(c, "download.originalFileName"), &v.outputName, false)
	})
	formRow(c, p, tr(c, "download.connectionLimit"), func() {
		f := rowInput(c, p, tr(c, "download.singleConnectionLimit"), tr(c, "common.auto"), &v.connectionLimit, false).Description(tr(c, "download.autoOrConnectionLimit"))
		if f.Changed() {
			v.FormError = ""
		}
	})
	ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(optionGap).Children(func() {
		ui.Text(c, tr(c, "download.mirrors")).Width(optionLabelWidth).Padding(8, 0, 0).FontSize(12).FontWeight(560).TextColor(p.muted)
		field := ui.TextAreaBase(c, &v.mirrorText).Transition(controlMotion).Grow(1).MinWidth(0).Lines(4, 4).Padding(8, 9).FontSize(13).FixedLineHeight(18).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.42)).FocusRing(true).Label(tr(c, "download.mirrorURLs")).Placeholder(tr(c, "download.mirrorPerLine"))
		if field.Focused() {
			field.Border(1, p.active)
		}
		if field.Changed() {
			v.FormError = ""
		}
	})
}
