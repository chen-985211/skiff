package presentation

import (
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
)

var controlMotion = ui.ElementTransition{Duration: 130 * time.Millisecond, Colors: true}
var panelMotion = ui.ElementTransition{
	Duration: 160 * time.Millisecond, Enter: &ui.Motion{Y: 5}, Exit: &ui.Motion{Y: -4},
}

type cardMatch string

func cardMotion(id string) ui.ElementTransition {
	return ui.ElementTransition{Match: cardMatch(id), ScaleContent: true, Duration: 260 * time.Millisecond, Enter: &ui.Motion{Y: 9, Scale: .985}, Exit: &ui.Motion{Y: -5, Scale: .985}}
}

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
	v.grid = ui.GridState{}
}

const (
	sidebarWidth = 174
	dockWidth    = 496
)

func (v *View) sidebar(c *ui.Context, p palette) {
	rail := ui.Column(c).Width(sidebarWidth).FillHeight().Shrink(0).Padding(0, 12, 18)
	rail.Children(func() {
		ui.Box(c).FillWidth().Height(max(20, c.TitleBar().Bottom)).DragWindow()
		ui.Row(c).Height(64).PaddingX(8).AlignItems(ui.Center).Gap(10).DragWindow().Children(func() {
			ui.Image(c, icons["boat"]).Size(38, 38).Fit(ui.Contain)
			ui.Text(c, "轻舟").FontSize(21).FontWeight(650)
		})
		const rowHeight, rowGap = 41, 5
		nav := ui.Column(c).Gap(rowGap)
		var selectedIndex int
		for i, cat := range categories {
			if cat.id == v.Filter {
				selectedIndex = i
				break
			}
		}
		offset := nav.Spring("selection", float32(selectedIndex*(rowHeight+rowGap)), ui.SpringOptions{Duration: 270 * time.Millisecond})
		nav.Children(func() {
			highlight := ui.Box(c.Key("highlight")).Absolute().Top(0).Left(0).Right(0).Height(rowHeight).Translate(0, offset).IgnoreInput(true).AccessHidden(true)
			if v.settingsOpen {
				highlight.Opacity(0)
			}
			surface(c, highlight, p, p.tint, 13, false)
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
							ui.Textf(c, "%d", count).Transition(panelMotion).ContentTransition(ui.ContentTransition{Numeric: true}).FontFeatures("tnum").FontSize(11).TextColor(ink)
						}
					})
				})
				if button.Clicked() {
					v.selectCategory(cat.id)
				}
			}
		})
		ui.Box(c).Grow(1).DragWindow()
		ui.Column(c).Gap(3).Children(func() {
			if v.Snapshot.Speed > 0 {
				ui.Row(c.Key("speed")).PaddingX(10).Gap(8).AlignItems(ui.Center).Transition(panelMotion).Children(func() {
					ui.Icon(c, icons["download"]).FontSize(14).TextColor(p.muted)
					ui.Text(c, bytesText(v.Snapshot.Speed)+"/s").FontSize(11).FontFeatures("tnum").ContentTransition(ui.ContentTransition{Numeric: true}).Description("总下载速度")
				})
			}
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
			settings := ui.ButtonBase(c.Key("settings")).FillWidth().Height(34).PaddingX(10).Radius(10).Transition(controlMotion).Label("设置").AccessSelected(v.settingsOpen)
			if v.settingsOpen || settings.Hovered() {
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

func (v *View) dock(c *ui.Context, p palette) {
	ui.Row(c).Height(80).Shrink(0).Center().Children(func() {
		bar := ui.Row(c).Width(dockWidth).Height(54).AlignItems(ui.Center).Padding(6, 7, 6, 18).Gap(10)
		surface(c, bar, p, p.tint, 27, true).Shadow(0, 4, 15, 0, c.Theme().Text.Alpha(.12)).Children(func() {
			ui.Icon(c, icons["search"]).FontSize(18).TextColor(p.muted)
			field := ui.TextInputBase(c, &v.Query).Label("搜索下载任务").Placeholder("搜索文件名或链接").Grow(1).MinWidth(0).Height(34).FontSize(13).KeyShortcuts(ui.Cmd, ui.KeyF)
			if !v.DialogOpen && !v.RestartOpen && c.Shortcut(ui.Cmd, ui.KeyF) {
				field.Focus()
			}
			cleared := false
			if v.Query != "" && (iconButton(c, "close", "清空搜索框", false) || field.Shortcut(0, ui.KeyEscape)) {
				v.Query = ""
				cleared = true
			}
			if cleared || field.Changed() {
				v.detailID = ""
				v.grid = ui.GridState{}
				c.Invalidate()
			}
			if primaryAction(c, "新建下载", "plus", !v.Ready || v.Busy).Width(108).Height(40).KeyShortcuts(ui.Cmd, ui.KeyN).Clicked() {
				v.OpenNew()
			}
		})
	})
}
