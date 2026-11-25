package presentation

import (
	"context"
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation/i18n"
)

func (v *View) queue(c *ui.Context, p palette) {
	width, _ := c.Size()
	available := max(float32(0), width-sidebarWidth-2*workspaceInset)
	columns := max(1, int((available+cardGap)/(cardWidth+cardGap)))
	gridWidth := min(available, float32(columns*(cardWidth+cardGap)-cardGap))
	if v.detailID != "" {
		for _, task := range v.Snapshot.Tasks {
			if task.ID == v.detailID {
				v.queueHeader(c, p, gridWidth, nil)
				v.taskDetail(c, p, task, columns)
				return
			}
		}
		v.detailID = ""
	}

	items := v.filtered()
	v.queueHeader(c, p, gridWidth, items)
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
		line := ui.Row(c).Gap(cardGap)
		line.Children(func() {
			for i := row * columns; i < min((row+1)*columns, len(items)); i++ {
				v.taskCard(c, p, items[i])
				if v.detailID == items[i].ID {
					viewport := v.taskRows.Bounds(c)
					// The row stays at its grid position while the card animates back into it.
					y := line.LayoutBounds().Y - viewport.Y
					if row == rows-1 || y+detailHeight > viewport.H {
						y -= cardHeight + cardGap
					}
					v.detailColumn = i % columns
					v.detailTop = max(0, min(y, viewport.H-detailHeight))
				}
			}
		})
	}).Width(gridWidth).Gap(cardGap).AlignSelf(ui.Start).Grow(1).MinHeight(0).Label(tr(c, "workspace.queue")).Transition(pageFade)
}

func (v *View) queueHeader(c *ui.Context, p palette, gridWidth float32, items []application.TaskDTO) {
	ui.Column(c.Key("queue-header")).Shrink(0).Children(func() {
		if v.detailID != "" {
			if ui.Box(c.Key("detail-header")).FillWidth().Height(workspaceHeader).Clicked() {
				v.detailID = ""
				c.Invalidate()
			}
		} else {
			ui.Row(c.Key("queue-toolbar")).Transition(panelMotion).AlignItems(ui.Center).Gap(9).Width(gridWidth).Height(workspaceHeader).Children(func() {
				ui.Box(c).Grow(1).FillHeight().DragWindow()
				if v.batchNotice != "" {
					ui.Text(c.Key("batch-notice"), v.batchNotice).Transition(pageFade).ContentTransition(textChange).SingleLine().FontSize(12).TextColor(p.muted).Tooltip(v.batchNotice)
				}
				inlineErrorMessage(c, queueErrorText(c.Locale(), v.queueError))
				v.batchAction(c, items, application.Pause)
				v.batchAction(c, items, application.Resume)
			})
		}
		if v.detailID == "" && strings.TrimSpace(v.Query) != "" {
			const resultGap = 12
			ui.Text(c, trf(c, "workspace.results", i18n.Args{"count": len(items)})).Transition(panelMotion).ContentTransition(numberChange).FontSize(12).TextColor(p.muted).IgnoreInput(true).Absolute().Left(0).Bottom(resultGap)
		}
	})
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
	action, icon := "workspace.resumeMany", "play"
	if cmd == application.Pause {
		action, icon = "workspace.pauseMany", "pause"
	}
	label := trf(c, action, i18n.Args{"count": len(ids)})
	t := c.Theme()
	b := ui.ButtonBase(c.Key(cmd)).Label(label).Disabled(v.Busy || !v.Ready).Height(32).PaddingX(12).Radius(cornerAction).Center().Transition(pageFade)
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
		filter, query := v.Filter, v.Query
		v.queueError = nil
		v.run(func(ctx context.Context) error { return v.API.SetPaused(ctx, ids, cmd == application.Pause) }, func(err error) {
			if v.Filter == filter && v.Query == query {
				v.queueError = err
			}
		})
	}
}

func queueErrorText(locale string, err error) string {
	batch, joined := err.(interface{ Unwrap() []error })
	var task *application.TaskActionError
	if !joined || !errors.As(err, &task) {
		return errorText(locale, err)
	}
	var messages []string
	for _, failure := range batch.Unwrap() {
		var task *application.TaskActionError
		if errors.As(failure, &task) {
			messages = append(messages, i18n.Format(locale, "workspace.taskActionFailed", i18n.Args{
				"name": task.TaskName, "reason": errorText(locale, task.Cause),
			}))
		} else {
			messages = append(messages, errorText(locale, failure))
		}
	}
	return strings.Join(messages, "\n")
}

func (v *View) empty(c *ui.Context, p palette, cat category) {
	title, icon := cat.empty, cat.icon
	switch {
	case !v.Ready && v.engineOffered:
		title, icon = "engine.missing", "download"
		if v.installingEngine {
			title = "engine.starting"
		}
		if v.EngineError != nil {
			title, icon = "engine.installFailed", "error"
		}
	case !v.Ready:
		title, icon = "engine.starting", "download"
		if v.EngineError != nil {
			title, icon = "engine.notReady", "error"
		}
	case strings.TrimSpace(v.Query) != "":
		title, icon = "workspace.noResults", "search"
	case cat.id == "all":
		icon = "tray"
	}
	ui.Column(c.Key(title)).Grow(1).MinHeight(0).FillWidth().Center().Gap(14).Transition(panelMotion).Children(func() {
		if icon == "tray" {
			ui.Image(c, inboxArt).Size(110, 110).Fit(ui.Contain)
		} else {
			ui.Icon(c, icons[icon]).FontSize(36).TextColor(p.muted)
		}
		ui.Text(c, tr(c, title)).FontSize(15).FontWeight(520).TextColor(p.muted)
		if !v.Ready {
			inlineError(c, v.EngineError)
		}
	})
}
