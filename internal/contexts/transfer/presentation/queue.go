package presentation

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

func (v *View) queue(c *ui.Context, p palette) {
	if v.detailID != "" {
		for _, task := range v.Snapshot.Tasks {
			if task.ID == v.detailID {
				v.messageBanner(c)
				v.taskDetail(c, p, task)
				return
			}
		}
		v.detailID = ""
	}

	items := v.filtered()
	width, _ := c.Size()
	available := max(float32(0), width-sidebarWidth-2*workspaceInset)
	columns := max(1, int((available+cardGap)/(cardWidth+cardGap)))
	gridWidth := min(available, float32(columns*(cardWidth+cardGap)-cardGap))
	ui.Column(c.Key("queue-header")).Shrink(0).Children(func() {
		ui.Row(c.Key("queue-toolbar")).Transition(panelMotion).AlignItems(ui.Center).Gap(9).Width(gridWidth).Height(workspaceHeader).Children(func() {
			ui.Box(c).Grow(1).FillHeight().DragWindow()
			v.batchAction(c, items, application.Pause)
			v.batchAction(c, items, application.Resume)
		})
		hasMessage := v.Message != ""
		v.messageBanner(c)
		if strings.TrimSpace(v.Query) != "" {
			const resultGap = 12
			count := ui.Textf(c, "%d 个结果", len(items)).Transition(panelMotion).ContentTransition(numberChange).FontSize(12).TextColor(p.muted).IgnoreInput(true)
			if hasMessage {
				count.AlignSelf(ui.Start).Margin(0, 0, resultGap)
			} else {
				count.Absolute().Left(0).Bottom(resultGap)
			}
		}
	})
	if len(items) == 0 {
		v.empty(c, p, v.category())
		return
	}
	if v.gridColumns > 0 && columns != v.gridColumns {
		if first, last := v.taskRows.Visible(); last >= first {
			v.taskRows.ScrollTo(first*v.gridColumns/columns, ui.Start)
		}
	}
	v.gridColumns = columns
	v.taskRows.Key = func(row int) any { return items[row*columns].ID }
	rows := (len(items) + columns - 1) / columns
	ui.List(c.Key(v.Filter), &v.taskRows, rows, func(row int) {
		ui.Row(c).Gap(cardGap).Children(func() {
			for i := row * columns; i < min((row+1)*columns, len(items)); i++ {
				v.taskCard(c, p, items[i])
			}
		})
	}).Width(gridWidth).Gap(cardGap).AlignSelf(ui.Start).Grow(1).MinHeight(0).Label("下载任务列表").Transition(panelMotion)
}

func (v *View) batchAction(c *ui.Context, items []application.TaskDTO, cmd application.Command) {
	var ids []string
	for _, t := range items {
		if cmd == application.Pause && t.CanPause || cmd == application.Resume && t.CanResume {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) < 2 {
		return
	}
	action, icon := "继续", "play"
	if cmd == application.Pause {
		action, icon = "暂停", "pause"
	}
	label := fmt.Sprintf("%s %d 项", action, len(ids))
	t := c.Theme()
	b := ui.ButtonBase(c.Key(cmd)).Label(label).Disabled(v.Busy || !v.Ready).Height(32).PaddingX(12).Radius(16).Center().Transition(pageFade)
	fill := t.Surface.Alpha(.40)
	if b.Pressed() {
		fill = t.SurfacePressed
	}
	b.Background(fill).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, label).SingleLine().FontSize(12).FontWeight(550).FontFeatures("tnum").ContentTransition(numberChange)
		})
	})
	if b.Clicked() {
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
		if action != "" && secondaryAction(c, action).Clicked() {
			switch {
			case strings.TrimSpace(v.Query) != "":
				v.Query = ""
				v.taskRows = ui.ListState{}
			default:
				v.selectCategory("all")
			}
		}
	})
}
