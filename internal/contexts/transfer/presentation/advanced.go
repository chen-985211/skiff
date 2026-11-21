package presentation

import "github.com/egoist/mygo/ui"

const (
	fileTab = iota
	headerTab
	proxyTab
)

type optionTabItem struct{ name, caption, icon string }

var optionTabs = [...]optionTabItem{
	{"文件与连接", "文件", "file"},
	{"请求头", "请求头", "headers"},
	{"代理", "代理", "network"},
}

const (
	optionLabelWidth = 82
	optionGap        = 8
	optionGutter     = 8 // the scroll thumb lives in the sheet margin
)

func (v *View) optionToolbar(c *ui.Context, p palette, width, ink float32) {
	optionTabBar(c, p, "高级下载选项", &v.optionTab, optionTabs[:], width).
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
			tab := tabs.Tab(i).ZIndex(1).Transition(controlMotion).Label(option.name).Tooltip(option.name).Width(cell).FillHeight().
				Center().Gap(5).Radius(cornerSmall).FocusRing(true).FocusRingOffset(-2)
			tint := tab.AnimateTo("selection-ink", amount(i == *selected), colorFade)
			color := p.muted.Mix(p.active, tint)
			if tab.Hovered() {
				color = c.Theme().Text
			}
			tab.Children(func() {
				ui.Icon(c, icons[option.icon]).FontSize(15).TextColor(color).AccessHidden(true)
				ui.Text(c, option.caption).SingleLine().FontSize(12).FontWeight(600).TextColor(color)
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
	formRow(c, p, "文件名", func() { rowInput(c, p, "单文件另存为", "使用原文件名", &v.outputName, false) })
	formRow(c, p, "连接上限", func() {
		f := rowInput(c, p, "单文件连接上限", "自动", &v.connectionLimit, false).Description("留空自动，也可输入 1 到 16")
		if f.Changed() {
			v.FormError = ""
		}
	})
	ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(optionGap).Children(func() {
		ui.Text(c, "备用地址").Width(optionLabelWidth).Padding(8, 0, 0).FontSize(12).FontWeight(560).TextColor(p.muted)
		field := ui.TextAreaBase(c, &v.mirrorText).Transition(controlMotion).Grow(1).MinWidth(0).Lines(4, 4).Padding(8, 9).FontSize(13).FixedLineHeight(18).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.42)).FocusRing(true).Label("备用下载地址").Placeholder("每行一个备用链接")
		if field.Focused() {
			field.Border(1, p.active)
		}
		if field.Changed() {
			v.FormError = ""
		}
	})
}
