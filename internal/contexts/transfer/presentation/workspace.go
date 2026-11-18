package presentation

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

type category struct{ id, title, icon, empty string }

var categories = []category{
	{"all", "全部任务", "all", "还没有下载任务"},
	{"active", "正在下载", "download", "当前没有正在下载的任务"},
	{"paused", "已暂停", "pause", "没有暂停的任务"},
	{"complete", "已完成", "check", "还没有已完成的任务"},
	{"error", "下载失败", "error", "没有失败的任务"},
}

func (v *View) category() category {
	for _, cat := range categories {
		if cat.id == v.Filter {
			return cat
		}
	}
	return categories[0]
}

func (v *View) selectCategory(id string) {
	v.settingsOpen = false
	v.Filter, v.detailID = id, ""
	v.taskRows = ui.ListState{}
}

const (
	sidebarWidth       = 174
	workspaceInset     = 26
	workspaceHeader    = 64
	settingsHeight     = 34
	footerInset        = 23
	dockHeight         = 44
	composeButtonWidth = 108
	cardWidth          = 336
	cardHeight         = 156
	cardGap            = 12
)

func (v *View) sidebar(c *ui.Context, p palette) {
	rail := ui.Column(c).Width(sidebarWidth).FillHeight().Shrink(0).Padding(0, 12, footerInset)
	rail.Children(func() {
		const rowHeight, rowGap = 41, 5
		top := max(float32(20), c.TitleBar().Bottom)
		_, height := c.Size()
		selectedY, selectedH := height-footerInset-settingsHeight, float32(settingsHeight)
		if !v.settingsOpen {
			selectedY, selectedH = top+workspaceHeader, rowHeight
			for i, cat := range categories {
				if cat.id == v.Filter {
					selectedY += float32(i * (rowHeight + rowGap))
					break
				}
			}
		}
		y := rail.AnimateTo("selection-y", selectedY, selectionSpring)
		h := rail.AnimateTo("selection-height", selectedH, selectionSpring)
		highlight := ui.Box(c.Key("selection")).Absolute().Left(12).Right(12).Top(0).Height(h).Translate(0, y).IgnoreInput(true).AccessHidden(true)
		surface(c, highlight, p, p.tint, 13, false)
		ui.Box(c).FillWidth().Height(max(20, c.TitleBar().Bottom)).DragWindow()
		ui.Row(c).Height(workspaceHeader).PaddingX(8).AlignItems(ui.Center).Gap(10).DragWindow().Children(func() {
			ui.Image(c, icons["boat"]).Size(38, 38).Fit(ui.Contain)
			ui.Text(c, "轻舟").FontSize(21).FontWeight(650)
		})
		ui.Column(c).ZIndex(1).Gap(rowGap).Children(func() {
			for _, cat := range categories {
				selected := !v.settingsOpen && v.Filter == cat.id
				button := ui.ButtonBase(c.Key(cat.id)).FillWidth().Height(rowHeight).ZIndex(1).PaddingX(11).Label(cat.title).AccessSelected(selected).Radius(13).Transition(controlMotion)
				ink := p.muted
				if selected {
					ink = c.Theme().Text
				} else if button.Hovered() {
					button.Background(p.panel.Alpha(.40))
				}
				button.Children(func() {
					ui.Row(c).Fill().AlignItems(ui.Center).Gap(9).Children(func() {
						ui.Icon(c, icons[cat.icon]).FontSize(17).TextColor(ink)
						ui.Text(c, cat.title).Grow(1).FontSize(13).FontWeight(570).TextColor(ink)
						if count := v.count(cat.id); count > 0 {
							ui.Textf(c, "%d", count).Transition(panelMotion).ContentTransition(numberChange).FontFeatures("tnum").FontSize(11).TextColor(ink)
						}
					})
				})
				if button.Clicked() {
					v.selectCategory(cat.id)
				}
			}
		})
		ui.Box(c).Grow(1).DragWindow()
		ui.Column(c).ZIndex(1).Gap(3).Children(func() {
			if dir := v.Snapshot.Directory; dir != "" {
				folder := ui.ButtonBase(c.Key("destination")).FillWidth().Height(34).Radius(10).PaddingX(10).Label("打开下载目录").Description(dir).Disabled(v.Actions.OpenDirectory == nil).Transition(controlMotion)
				if folder.Hovered() {
					folder.Background(p.panel.Alpha(.38))
				}
				folder.Children(func() {
					ui.Row(c).Fill().AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Icon(c, icons["folder"]).FontSize(16).TextColor(p.muted)
						ui.Text(c, filepath.Base(dir)).Grow(1).MinWidth(0).FontSize(12).SingleLine()
					})
				})
				if folder.Clicked() {
					v.Actions.OpenDirectory(dir)
				}
			}
			settings := ui.ButtonBase(c.Key("settings")).FillWidth().Height(settingsHeight).PaddingX(10).Radius(10).Transition(controlMotion).Label("设置").AccessSelected(v.settingsOpen)
			if !v.settingsOpen && settings.Hovered() {
				settings.Background(p.panel.Alpha(.48))
			}
			settings.Children(func() {
				ui.Row(c).Fill().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icons["settings"]).FontSize(16).TextColor(p.muted)
					ui.Text(c, "设置").Grow(1).FontSize(12)

				})
			})
			if settings.Clicked() {
				v.settingsOpen = true
				c.Invalidate()
			}
		})
	})
}

func (v *View) dock(c *ui.Context, p palette) (search, trigger ui.Element) {
	ui.Row(c.Key("task-dock")).Transition(dockMotion).Height(cardGap+dockHeight+footerInset).Shrink(0).Padding(0, 0, footerInset).AlignItems(ui.End).Justify(ui.Start).Children(func() {
		ui.Row(c).Role(ui.RoleToolbar).Label("下载工具栏").Width(cardWidth).MaxWidthPercent(100).Height(dockHeight).Gap(cardGap).Children(func() {
			search = ui.Row(c).Role(ui.RoleSearch).Label("任务搜索").Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Padding(0, 8, 0, 16).Gap(8)
			surface(c, search, p, p.tint, dockHeight/2, true).Shadow(0, 3, 10, 0, c.Theme().Text.Alpha(.10)).Children(func() {
				ui.Icon(c, icons["search"]).FontSize(18).TextColor(p.muted).IgnoreInput(true)
				field := ui.TextInputBase(c, &v.Query).Label("搜索下载任务").Placeholder("搜索文件名或链接").Grow(1).MinWidth(0).FillHeight().FontSize(13).KeyShortcuts(ui.Cmd, ui.KeyF)
				if search.Clicked() || (!v.DialogOpen && !v.RestartOpen && c.Shortcut(ui.Cmd, ui.KeyF)) {
					field.Focus()
				}
				cleared := false
				if v.Query != "" && (iconButton(c, "close", "清空搜索框", false) || field.Shortcut(0, ui.KeyEscape)) {
					v.Query = ""
					cleared = true
					field.Focus()
				}
				if cleared || field.Changed() {
					v.detailID = ""
					v.taskRows = ui.ListState{}
					c.Invalidate()
				}
			})
			trigger = primaryAction(c, "新建下载", "plus", !v.Ready || v.Busy).Width(composeButtonWidth).Shrink(0).FillHeight().Radius(dockHeight/2).KeyShortcuts(ui.Cmd, ui.KeyN)
			if trigger.Clicked() {
				v.OpenNew()
			}
		})
	})
	return search, trigger
}
