package presentation

import "github.com/egoist/mygo/ui"

const (
	fileTab = iota
	headerTab
	proxyTab
)

var optionTitles = [...]string{"文件与连接", "请求头", "代理"}

// Tabs share one viewport. Switching to a shorter page keeps the tab under
// the pointer and the download action anchored at the bottom of the sheet.
func (v *View) advancedOptions(c *ui.Context, p palette, width, linkHeight, reveal float32) {
	_, windowH := c.Size()
	// Reserve the title bar and the measured link editor before sizing the scroll area.
	const frameHeight = 2*formInsetY + formHeaderHeight + 2*formGap + formFooterHeight
	available := windowH - max(float32(36), c.TitleBar().Bottom+8) - footerInset - frameHeight - max(64, linkHeight)
	if v.FormError != "" {
		available -= formErrorMaxHeight + formGap
	}
	const navigationHeight = 48
	bodyHeight := min(float32(196), max(64, available-navigationHeight))
	slot := ui.Column(c.Key("advanced-slot")).FillWidth().Height((navigationHeight + bodyHeight) * reveal).Clip().Opacity(reveal).AccessHidden(!v.advanced).IgnoreInput(!v.advanced).Disabled(v.Busy || !v.advanced)
	if reveal == 0 && !v.advanced {
		return
	}
	slot.Children(func() {
		tabs := ui.TabsBase(c, &v.optionTab, len(optionTitles))
		tabWidth := width / float32(len(optionTitles))
		rail := tabs.List.FillWidth().Margin(8, 0, 8).Height(32).Label("高级下载选项")
		x := rail.AnimateTo("active-tab", float32(v.optionTab)*tabWidth, selectionSpring)
		rail.Children(func() {
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border.Alpha(.55)).IgnoreInput(true)
			ui.Box(c.Key("active-tab")).Absolute().Left(10).Bottom(0).Size(tabWidth-20, 3).Translate(x, 0).Radius(cornerProgress).Background(p.active).IgnoreInput(true).AccessHidden(true)
			for i, title := range optionTitles {
				tab := tabs.Tab(i).Transition(controlMotion).Width(tabWidth).FillHeight().Center().Radius(cornerSmall).Label(title).FocusRing(true)
				ink := p.muted
				if i == v.optionTab {
					ink = c.Theme().Text
				}
				if tab.Hovered() {
					tab.Background(p.panel.Alpha(.24))
				}
				tab.Children(func() { ui.Text(c, title).SingleLine().FontSize(13).FontWeight(590).TextColor(ink) })
			}
		})
		// Only the selected editor is focusable. The scroll surface is keyed by
		// page so an overflowing header list never offsets a new page.
		scroll := ui.Scroll(c.Key(optionTitles[v.optionTab])).Height(bodyHeight).FillWidth().Clip()
		if !c.Preferences().ReduceMotion {
			scroll.Transition(pageFade)
		}
		scroll.Children(func() {
			page := ui.Column(c).FillWidth().Gap(7).Padding(3, 8, 3, 0)
			page.Children(func() {
				switch v.optionTab {
				case fileTab:
					v.fileOptions(c, p)
				case headerTab:
					v.headerOptions(c, p)
				case proxyTab:
					v.proxyOptions(c, p, &v.taskNetwork, true, width-8)
				}
			})
		})
	})
}

func formRow(c *ui.Context, p palette, label string, control func()) {
	ui.Row(c.Key(label)).FillWidth().Height(32).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).Width(82).FontSize(12).FontWeight(560).TextColor(p.muted).SingleLine()
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
	ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(8).Children(func() {
		ui.Text(c, "备用地址").Width(82).Padding(8, 0, 0).FontSize(12).FontWeight(560).TextColor(p.muted)
		field := ui.TextAreaBase(c, &v.mirrorText).Transition(controlMotion).Grow(1).MinWidth(0).Lines(3, 4).Padding(8, 9).FontSize(13).FixedLineHeight(18).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.42)).FocusRing(true).Label("备用下载地址").Placeholder("同一文件的备用链接，每行一个")
		if field.Focused() {
			field.Border(1, p.active)
		}
		if field.Changed() {
			v.FormError = ""
		}
	})
}
