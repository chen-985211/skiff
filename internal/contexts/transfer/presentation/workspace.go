package presentation

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

type category struct{ id, title, icon, empty string }

var categories = []category{
	{"all", "workspace.all", "all", "workspace.allEmpty"},
	{"active", "workspace.active", "download", "workspace.activeEmpty"},
	{"paused", "task.paused", "pause", "workspace.pausedEmpty"},
	{"complete", "task.complete", "check", "workspace.completedEmpty"},
	{"error", "task.failed", "error", "workspace.failedEmpty"},
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
	if v.settingsOpen && v.concurrencyEdited {
		v.commitConcurrency(false)
	}
	v.settingsOpen = false
	v.Filter, v.detailID = id, ""
	v.batchNotice = messageError{}
	v.queueError = nil
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
		surface(c, highlight, p, cornerRow, selectionFrost)
		ui.Box(c).FillWidth().Height(max(20, c.TitleBar().Bottom)).DragWindow()
		ui.Row(c).Height(workspaceHeader).PaddingX(8).AlignItems(ui.Center).Gap(10).DragWindow().Children(func() {
			ui.Image(c, icons["boat"]).Size(38, 38).Fit(ui.Contain)
			ui.Text(c, tr(c, "common.appName")).FontSize(21).FontWeight(650)
		})
		ui.Column(c).ZIndex(1).Gap(rowGap).Children(func() {
			for _, cat := range categories {
				selected := !v.settingsOpen && v.Filter == cat.id
				button := ui.ButtonBase(c.Key(cat.id)).FillWidth().Height(rowHeight).ZIndex(1).PaddingX(11).Label(tr(c, cat.title)).AccessSelected(selected).Radius(cornerRow).Transition(controlMotion)
				ink := p.muted
				if selected {
					ink = c.Theme().Text
				} else if button.Hovered() {
					button.Background(p.panel.Alpha(.40))
				}
				button.Children(func() {
					ui.Row(c).Fill().AlignItems(ui.Center).Gap(9).Children(func() {
						ui.Icon(c, icons[cat.icon]).FontSize(17).TextColor(ink)
						ui.Text(c, tr(c, cat.title)).Grow(1).FontSize(13).FontWeight(570).TextColor(ink)
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
		if v.Ready {
			inlineError(c, v.EngineError)
		}
		ui.Box(c).Grow(1).DragWindow()
		ui.Column(c).ZIndex(1).Gap(3).Children(func() {
			if dir := v.Snapshot.Directory; dir != "" {
				folder := ui.ButtonBase(c.Key("destination")).FillWidth().Height(34).Radius(cornerCompact).PaddingX(10).Label(tr(c, "workspace.folder")).Description(dir).Disabled(v.Actions.OpenDirectory == nil).Transition(controlMotion)
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
					v.openDirectoryError = nil
					v.Actions.OpenDirectory(dir, func(err error) {
						if v.Snapshot.Directory == dir {
							v.openDirectoryError = err
						}
					})
				}
				inlineError(c, v.openDirectoryError)
			}
			settings := ui.ButtonBase(c.Key("settings")).FillWidth().Height(settingsHeight).PaddingX(10).Radius(cornerCompact).Transition(controlMotion).Label(tr(c, "common.settings")).AccessSelected(v.settingsOpen)
			if !v.settingsOpen && settings.Hovered() {
				settings.Background(p.panel.Alpha(.48))
			}
			settings.Children(func() {
				ui.Row(c).Fill().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icons["settings"]).FontSize(16).TextColor(p.muted)
					ui.Text(c, tr(c, "common.settings")).Grow(1).FontSize(12)

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
		ui.Row(c).Role(ui.RoleToolbar).Label(tr(c, "workspace.toolbar")).Width(cardWidth).MaxWidthPercent(100).Height(dockHeight).Gap(cardGap).Children(func() {
			search = ui.Row(c).Role(ui.RoleSearch).Label(tr(c, "workspace.searchRegion")).Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Padding(0, 8, 0, 16).Gap(8)
			surface(c, search, p, cornerDock, searchGlass).Shadow(0, 3, 10, 0, c.Theme().Text.Alpha(.10)).Children(func() {
				ui.Icon(c, icons["search"]).FontSize(18).TextColor(p.muted).IgnoreInput(true)
				field := ui.TextInputBase(c, &v.Query).Transition(controlMotion).Label(tr(c, "workspace.searchLabel")).Placeholder(tr(c, "workspace.searchPlaceholder")).Grow(1).MinWidth(0).FillHeight().FontSize(13).KeyShortcuts(ui.Cmd, ui.KeyF)
				if search.Clicked() || (!v.DialogOpen && !v.RestartOpen && c.Shortcut(ui.Cmd, ui.KeyF)) {
					field.Focus()
				}
				cleared := false
				if v.Query != "" && (iconButton(c, "close", tr(c, "workspace.searchClear"), false) || field.Shortcut(0, ui.KeyEscape)) {
					v.Query = ""
					cleared = true
					field.Focus()
				}
				if cleared || field.Changed() {
					v.detailID = ""
					v.queueError = nil
					v.taskRows = ui.ListState{}
					c.Invalidate()
				}
			})
			if !v.Ready && v.engineOffered {
				label := tr(c, "engine.download")
				if v.installingEngine {
					label = tr(c, "engine.downloading")
				}
				trigger = primaryAction(c, label, "download", v.installingEngine || v.Actions.DownloadEngine == nil)
				if trigger.Clicked() && !v.installingEngine && v.Actions.DownloadEngine != nil {
					v.installingEngine = true
					v.EngineError = nil
					v.Actions.DownloadEngine()
				}
			} else {
				trigger = primaryAction(c, tr(c, "download.new"), "plus", !v.Ready || v.Busy).KeyShortcuts(ui.Cmd, ui.KeyN)
				if trigger.Clicked() {
					v.OpenNew()
				}
			}
			width := primaryActionWidth(c, composeButtonWidth, tr(c, "download.new"), tr(c, "engine.download"), tr(c, "engine.downloading"))
			trigger.Width(width).Shrink(0).FillHeight().Radius(cornerDock)
		})
	})
	return search, trigger
}
