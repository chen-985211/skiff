package presentation

import (
	"fmt"
	"path"
	"skiff/internal/contexts/transfer/presentation/i18n"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

const (
	detailHeight  = 2*cardHeight + cardGap
	cardInset     = 16
	fileIconSize  = 44
	fileColumnGap = 12
)

// The footer keeps the same geometry when an action becomes a status.
func cardPill(c *ui.Context, pill ui.Element, icon, text, status string, change ui.ContentTransition) {
	ink := c.Theme().Text
	if status == "error" {
		ink = c.Theme().Danger
	}
	pill.WidthPercent(25).MinWidth(80).MaxWidth(104).Height(32).Radius(cornerField).Center().Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
			ui.Icon(c, icons[icon]).TextColor(ink).FontSize(14).ContentTransition(change)
			ui.Text(c, text).FontSize(11).FontWeight(550).ContentTransition(change)
		})
	})
}

func cardAction(c *ui.Context, icon, text, label, status string, change ui.ContentTransition) bool {
	key := label
	if status != "" {
		key = "primary-action"
	}
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Transition(controlMotion)
	b.Background(c.Theme().Surface.Alpha(.38))
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	if b.Pressed() {
		b.Background(c.Theme().SurfacePressed)
	}
	if status != "" {
		b.Description(statusText(c, status))
		if !b.Hovered() && !b.Focused() {
			text, icon = statusText(c, status), stateSymbol(status)
		}
	}
	cardPill(c, b, icon, text, status, change)
	return b.Clicked()
}

func (v *View) rowActions(c *ui.Context, t application.TaskDTO, displayStatus string, change ui.ContentTransition) {
	if v.Busy {
		return
	}
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		var cmd application.Command
		label, text, icon := tr(c, "task.fileDetails"), tr(c, "task.details"), "file"
		switch {
		case t.CanPause:
			label, text, icon, cmd = tr(c, "task.pause"), tr(c, "common.pause"), "pause", application.Pause
		case t.CanResume:
			label, text, icon, cmd = tr(c, "task.resume"), tr(c, "common.resume"), "play", application.Resume
		case t.NeedsFTPLogin:
			label, text, icon = tr(c, "ftp.signIn"), tr(c, "ftp.signIn"), "download"
		case t.CanRetry:
			label, text, icon, cmd = tr(c, "task.retry"), tr(c, "task.retry"), "retry", application.Retry
		}
		if cmd == "" && !t.NeedsFTPLogin && v.detailID == t.ID {
			state := statusText(c, displayStatus)
			badge := ui.Box(c.Key("task-status")).Role(ui.RoleStatus).Label(state).Background(c.Theme().Surface.Alpha(.38))
			cardPill(c, badge, stateSymbol(displayStatus), state, displayStatus, change)
		} else if cardAction(c, icon, text, label, displayStatus, change) {
			if t.NeedsFTPLogin {
				v.ftpID = t.ID
				v.ftpOpen = true
				v.ftpUser = ""
				v.ftpPassword = ""
				c.Invalidate()
			} else if cmd != "" {
				v.command(t.ID, cmd)
			} else {
				v.detailID = t.ID
				c.Invalidate()
			}
		}
		if t.CanPrioritize && cardAction(c, "download", tr(c, "task.prioritize"), tr(c, "task.prioritize"), "", textChange) {
			v.command(t.ID, application.Prioritize)
		}
		if t.NextInQueue {
			ui.Text(c.Key("next-in-queue"), tr(c, "task.priority")).Transition(pageFade).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if t.CanRestart && cardAction(c, "restart", tr(c, "task.restart"), tr(c, "task.restart"), "", textChange) {
			v.RestartID, v.RestartOpen = t.ID, true
		}
		if t.Path != "" && v.Actions.Reveal != nil && cardAction(c, "folder", tr(c, "task.location"), tr(c, "task.reveal"), "", textChange) {
			v.setTaskError(t.ID, v.Actions.Reveal(t.Path))
			c.Invalidate()
		}
	})
}

func (v *View) removeTaskButton(c *ui.Context, id string) {
	if v.Busy {
		return
	}
	label := tr(c, "task.remove")
	b := ui.ButtonBase(c.Key("remove-task")).Transition(controlMotion).Absolute().Top(8).Right(8).ZIndex(2).Size(30, 30).Center().Radius(cornerRemove).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, icons["close"]).FontSize(14).TextColor(c.Theme().TextMuted) })
	if b.Clicked() {
		v.command(id, application.Remove)
	}
}

func fileIcon(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".zip", ".gz", ".tar", ".7z", ".rar", ".dmg", ".iso":
		return "archive"
	case ".mp3", ".flac", ".wav", ".m4a", ".ogg":
		return "audio"
	case ".mp4", ".mov", ".mkv", ".webm":
		return "media"
	default:
		return "file"
	}
}

func taskColor(p palette, status string) ui.Color {
	switch status {
	case "active":
		return p.active
	case "complete":
		return p.complete
	case "paused":
		return p.paused
	case "waiting":
		return p.waiting
	case "error":
		return p.muted
	default:
		return p.muted
	}
}

func fileHeading(c *ui.Context, p palette, t application.TaskDTO, titleSize, lineGap float32) {
	ui.Image(c, fileArt[fileIcon(t.Name)]).Size(fileIconSize, fileIconSize).Fit(ui.Contain)
	ui.Column(c).Grow(1).MinWidth(0).Gap(lineGap).Children(func() {
		ui.Text(c, t.Name).FontSize(titleSize).FontWeight(640).SingleLine().TruncateMiddle().Tooltip(t.Name)
		if size := taskSize(t); size != "" {
			ui.Text(c, size).Transition(pageFade).ContentTransition(numberChange).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
		}
	})
}

func (v *View) taskCard(c *ui.Context, p palette, t application.TaskDTO) {
	card := ui.Column(c.Key(t.ID)).Size(cardWidth, cardHeight).MaxWidthPercent(100).Shrink(0).Role(ui.RoleGroup).Label(t.Name).Padding(12, cardInset).Gap(4).Transition(cardMotion(t.ID))
	progress := v.cardProgress(c, card, t)
	change := progress.statusChange()
	color := card.AnimateColor("status", taskColor(p, progress.status), *change.Animation)
	v.cardSurface(c, card, p, t.ID, color, cornerCard).Children(func() {
		head := ui.ButtonBase(c).Label(trf(c, "task.openDetails", i18n.Args{"name": t.Name})).Tooltip(t.Name + " · " + statusText(c, t.Status)).Height(54).FillWidth().Radius(cornerRow).Transition(controlMotion)
		head.Children(func() {
			ui.Row(c).Fill().Gap(fileColumnGap).AlignItems(ui.Center).Children(func() {
				fileHeading(c, p, t, 16, 4)
				ui.Box(c).Width(23).Shrink(0)
			})
		})
		if head.Clicked() || card.Clicked() {
			v.detailID = t.ID
			c.Invalidate()
		}
		v.taskProgress(c, p, t, color, progress)
		ui.Box(c).Grow(1)
		v.rowActions(c, t, progress.status, change)
		v.removeTaskButton(c, t.ID)
	})
}

func stateSymbol(status string) string {
	switch status {
	case "active":
		return "download"
	case "paused":
		return "pause"
	case "complete":
		return "check"
	case "error":
		return "error"
	default:
		return "clock"
	}
}

func taskSize(t application.TaskDTO) string {
	if t.Status == "complete" {
		return bytesText(max(t.Total, t.Completed))
	}
	if t.Total > 0 {
		return bytesText(t.Completed) + " / " + bytesText(t.Total)
	}
	if t.Completed > 0 {
		return bytesText(t.Completed)
	}
	return ""
}

func (v *View) taskProgress(c *ui.Context, p palette, t application.TaskDTO, color ui.Color, progress progressFrame) {
	actionError := v.taskErrors[t.ID]
	if progress.opacity == 0 {
		inlineError(c, actionError)
		return
	}
	active := t.Status == "active"

	section := ui.Column(c.Key("progress")).Gap(6).Transition(pageFade).Opacity(progress.opacity)
	fraction := progress.fraction
	known := t.Total > 0 || t.Status == "complete"
	section.Children(func() {
		ui.Row(c).Height(18).AlignItems(ui.Center).Children(func() {
			metric, ink := "", p.muted
			if actionError != nil {
				metric, ink = errorText(c.Locale(), actionError), c.Theme().Danger
			} else if reason := taskFailureText(c.Locale(), t); reason != "" {
				metric, ink = reason, c.Theme().Danger
			} else if t.Seeding {
				metric = tr(c, "task.seeding")
			} else if active && t.Speed > 0 {
				metric = bytesText(t.Speed) + "/s"
			}
			ui.Text(c, metric).MinWidth(0).Grow(1).SingleLine().Tooltip(metric).FontSize(12).FontFeatures("tnum").TextColor(ink).ContentTransition(textChange)
			if known {
				ui.Textf(c, "%.0f%%", 100*fraction).Transition(pageFade).FontSize(12).FontWeight(600).FontFeatures("tnum")
			}
		})
		track := ui.Box(c).Height(5).FillWidth().Radius(cornerProgress).Background(p.border).Clip().Role(ui.RoleProgress).Label(tr(c, "task.progress"))
		if known {
			track.Range(0, 1, float64(fraction))
		} else {
			track.Description(tr(c, "task.unknownSize"))
		}
		// Only the visible track requests paint frames. Counters and layout
		// continue to update on actual engine observations.
		target := float32(0)
		if active && !c.Preferences().ReduceMotion && (!known || t.Speed > 0 && fraction > 0) {
			target = 1
		}
		strength := track.AnimateTo("flow", target, progressEase)
		track.Draw(func(painter *ui.Painter, r ui.Rect) {
			fill := r
			if known {
				fill.W *= max(0, min(1, fraction))
				painter.Fill(fill, color, cornerProgress)
			}
			if strength <= 0 {
				if active && !known {
					// Three centered dots indicate an unknown length without
					// suggesting a measured fraction of the file.
					for i := range 3 {
						painter.Fill(ui.Rect{X: r.X + r.W/2 + float32(i-1)*9 - 2.5, Y: r.Y, W: 5, H: r.H}, color, cornerProgress)
					}
				}
				return
			}
			painter.AnimationFrame()
			phase := float32(painter.Now().UnixMilli()%1650) / 1650
			band := min(fill.W*.6, 64)
			x := fill.X - band + phase*(fill.W+band)
			shine := color.Mix(p.panel, .65)
			if !known {
				shine = color
			}
			painter.Clip(fill, cornerProgress, func() {
				painter.FillGradient(ui.Rect{X: x, Y: r.Y, W: band, H: r.H}, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
					{Color: shine.Alpha(0), At: 0}, {Color: shine.Alpha(strength), At: .55}, {Color: shine.Alpha(0), At: 1},
				}}, cornerProgress)
			})
		})
	})
}

func (v *View) taskDetail(c *ui.Context, p palette, t application.TaskDTO, columns int) {
	ui.Column(c.Key("detail")).Grow(1).MinHeight(0).Children(func() {
		backdrop := ui.Box(c.Key("detail-dismiss")).Absolute().Top(0).Left(0).Right(0).Bottom(0).AccessHidden(true)
		if backdrop.Clicked() || c.Shortcut(0, ui.KeyEscape) {
			v.detailID = ""
			c.Invalidate()
		}
		column := min(v.detailColumn, max(0, columns-2))
		ui.Box(c).Height(v.detailTop).MinHeight(0).Shrink(1).IgnoreInput(true)
		card := ui.Column(c.Key(t.ID)).Size(2*cardWidth+cardGap, detailHeight).Shrink(0).Margin(0, 0, 0, float32(column*(cardWidth+cardGap))).
			MaxWidthPercent(100).MaxHeightPercent(100).AlignSelf(ui.Start).ZIndex(1).Role(ui.RoleGroup).Label(t.Name).Padding(16, cardInset).Gap(16).Transition(cardMotion(t.ID))
		progress := v.cardProgress(c, card, t)
		change := progress.statusChange()
		color := card.AnimateColor("status", taskColor(p, progress.status), *change.Animation)
		v.cardSurface(c, card, p, t.ID, color, cornerDetail).Children(func() {
			ui.Row(c).Height(44).FillWidth().Gap(fileColumnGap).AlignItems(ui.Center).Children(func() {
				fileHeading(c, p, t, 18, 2)
				if t.Status != "complete" {
					v.speedSelector(c, p, t.ID, t.DownloadLimit)
				}
				if secondaryAction(c, tr(c, "common.back")).Label(tr(c, "task.backToQueue")).Clicked() {
					v.detailID = ""
					c.Invalidate()
				}
			})
			ui.Column(c).Grow(1).MinHeight(0).Gap(16).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					source := t.URL
					if t.MirrorSource != "" {
						source = t.MirrorSource
					}
					detailField(c, p, tr(c, "task.source"), source)
					target := t.Path
					if target == "" {
						target = t.Directory
					}
					detailField(c, p, tr(c, "task.location"), target)
					if t.TorrentHash != "" {
						if secondaryAction(c, tr(c, "bt.details")).Clicked() {
							v.showBTDetails(t.ID)
						}
					} else if t.Status == "active" && t.ConnectionLimit > 0 {
						detailField(c, p, tr(c, "common.connection"), fmt.Sprintf("%d / %d", t.Connections, t.ConnectionLimit))
					}
				})
				v.taskProgress(c, p, t, color, progress)
			})
			v.rowActions(c, t, progress.status, change)
		})
	})
}

func detailField(c *ui.Context, p palette, label, value string) {
	ui.Row(c.Key(label)).Transition(pageFade).AlignItems(ui.Center).Gap(fileColumnGap).Children(func() {
		ui.Text(c, label).Width(fileIconSize).FontSize(12).TextColor(p.muted)
		ui.Text(c, value).ContentTransition(textChange).FontSize(12).Grow(1).MinWidth(0).SingleLine().Selectable().Tooltip(value)
	})
}
