package presentation

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

func (v *View) queue(c *ui.Context, p palette) {
	if v.detailID != "" {
		for _, task := range v.Snapshot.Tasks {
			if task.ID == v.detailID {
				v.taskDetail(c, p, task)
				return
			}
		}
		v.detailID = ""
	}

	items := v.filtered()
	toolbar := ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(9).Height(40).Padding(0, 4)
	toolbar.Children(func() {
		if strings.TrimSpace(v.Query) != "" {
			ui.Textf(c, "%d 个结果", len(items)).Transition(panelMotion).ContentTransition(ui.ContentTransition{Numeric: true}).FontSize(12).TextColor(p.muted)
		}
		ui.Box(c).Grow(1).FillHeight().DragWindow()
		v.batchAction(c, items, application.Pause)
		v.batchAction(c, items, application.Resume)
	})
	if len(items) == 0 {
		v.empty(c, p, v.category())
		return
	}
	v.grid.Key = func(i int) any { return items[i].ID }
	v.grid.Label = func(i int) string { return items[i].Name }
	// GridView estimates columns from the full window on its first frame.
	// Account for the rail before it has measured the workspace.
	width, _ := c.Size()
	const minCardWidth = 350
	minWidth := float32(minCardWidth)
	if width-sidebarWidth-48 < 2*minCardWidth+8 {
		minWidth = width
	}
	ui.GridView(c.Key(v.Filter), &v.grid, len(items), minWidth, 194, func(i int) { v.taskCard(c, p, items[i]) }).Grow(1).MinHeight(0).Label("下载任务列表").Transition(panelMotion)
}

func (v *View) batchAction(c *ui.Context, items []application.TaskDTO, cmd application.Command) {
	var ids []string
	for _, t := range items {
		if cmd == application.Pause && t.CanPause || cmd == application.Resume && t.CanResume {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	label, icon := "继续当前结果", "play"
	if cmd == application.Pause {
		label, icon = "暂停当前结果", "pause"
	}
	if iconButton(c, icon, label, v.Busy || !v.Ready) {
		v.run("", func(ctx context.Context) error { return v.API.SetPaused(ctx, ids, cmd == application.Pause) })
	}
}

func (v *View) empty(c *ui.Context, p palette, cat category) {
	title, icon := cat.empty, cat.icon
	action := ""
	switch {
	case !v.Ready:
		title, icon = "正在准备下载引擎", "download"
		if v.Message != "" {
			title, icon = "下载引擎未就绪", "error"
		}
	case strings.TrimSpace(v.Query) != "":
		title, icon, action = "没有匹配的任务", "search", "清除搜索"
	case cat.id == "all":
		icon = "tray"
	case len(v.Snapshot.Tasks) > 0:
		action = "查看全部任务"
	}
	ui.Column(c.Key(title)).Grow(1).MinHeight(0).FillWidth().Center().Gap(14).Transition(panelMotion).Children(func() {
		if icon == "tray" {
			ui.Image(c, inboxArt).Size(110, 110).Fit(ui.Contain)
		} else {
			ui.Icon(c, icons[icon]).FontSize(36).TextColor(p.muted)
		}
		ui.Text(c, title).FontSize(15).FontWeight(520).TextColor(p.muted)
		if action != "" && ui.Button(c, action).Height(32).PaddingX(13).Radius(8).FontSize(12).Clicked() {
			switch {
			case strings.TrimSpace(v.Query) != "":
				v.Query = ""
				v.grid = ui.GridState{}
			default:
				v.selectCategory("all")
			}
		}
	})
}
