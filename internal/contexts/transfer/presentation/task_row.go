package presentation

import (
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

// The four controls keep the same order, width and hit target in every state.
func cardAction(c *ui.Context, icon, text, label string, disabled bool) bool {
	b := ui.ButtonBase(c).Basis(0).Grow(1).MinWidth(0).Height(32).Radius(12).Center().Label(label).Tooltip(label).Disabled(disabled).Transition(controlMotion)
	b.Background(c.Theme().Surface.Alpha(.38))
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	if b.Pressed() {
		b.Background(c.Theme().SurfacePressed)
	}
	b.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(14).ContentTransition(ui.ContentTransition{})
			ui.Text(c, text).FontSize(11).FontWeight(550).ContentTransition(ui.ContentTransition{})
		})
	})
	return b.Clicked()
}

func (v *View) rowActions(c *ui.Context, t application.TaskDTO) {
	ui.Row(c).FillWidth().Gap(4).Children(func() {
		var cmd application.Command
		label, text, icon := "查看文件详情", "详情", "file"
		switch {
		case t.CanPause:
			label, text, icon, cmd = "暂停下载", "暂停", "pause", application.Pause
		case t.CanResume:
			label, text, icon, cmd = "继续下载", "继续", "play", application.Resume
		case t.CanRetry:
			label, text, icon, cmd = "重试下载", "重试下载", "retry", application.Retry
		case v.detailID == t.ID:
			label, text, icon = "返回任务列表", "返回", "chevron"
		}
		if cardAction(c, icon, text, label, v.Busy) {
			if cmd != "" {
				v.command(t.ID, cmd)
			} else {
				if v.detailID == t.ID {
					v.detailID = ""
				} else {
					v.detailID = t.ID
				}
				c.Invalidate()
			}
		}
		if cardAction(c, "restart", "从头下载", "从头下载", !t.CanRestart || v.Busy) {
			v.RestartID, v.RestartOpen = t.ID, true
		}
		if cardAction(c, "folder", "位置", "在文件夹中显示", t.Path == "" || v.Busy) && v.Actions.Reveal != nil {
			v.Actions.Reveal(t.Path)
		}
		if cardAction(c, "close", "移除", "移除任务（保留文件）", v.Busy) {
			v.command(t.ID, application.Remove)
		}
	})
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

func taskColor(c *ui.Context, p palette, status string) ui.Color {
	switch status {
	case "active":
		if c.Theme().Dark {
			return ui.Hex("#b0cfed")
		}
		return ui.Hex("#315d85")
	case "complete":
		return p.green
	case "error":
		return c.Theme().Danger.Mix(c.Theme().Text, .22)
	default:
		return p.muted
	}
}

func (v *View) taskCard(c *ui.Context, p palette, t application.TaskDTO) {
	card := ui.Column(c.Key(t.ID)).Fill().Padding(15, 17, 12).Gap(8).Transition(cardMotion(t.ID))
	color := card.AnimateColor("status", taskColor(c, p, t.Status), ui.Animation{Duration: 230 * time.Millisecond})
	kind := fileIcon(t.Name)
	plate := surface(c, card, p, p.tint, 20, false)
	if !c.Preferences().HighContrast && !c.Preferences().ReduceTransparency {
		reflected := p.tint.Mix(color, .32)
		if c.Theme().Dark {
			reflected = reflected.Mix(p.panel, .70)
		}
		plate.LinearGradient(ui.LinearGradient{Angle: 152, Stops: []ui.GradientStop{
			{Color: reflected.Alpha(.51), At: 0}, {Color: p.panel.Alpha(.76), At: .42},
			{Color: p.panel.Alpha(.60), At: .73}, {Color: reflected.Mix(p.panel, .52).Alpha(.56), At: 1},
		}})
	}
	plate.Children(func() {
		head := ui.ButtonBase(c).Label("查看详情 " + t.Name).Tooltip(t.Name + " · " + statusText(t.Status)).Height(71).FillWidth().Radius(13).Transition(controlMotion)
		if head.Hovered() {
			head.Background(p.panel.Alpha(.58))
		}
		head.Children(func() {
			ui.Row(c).Fill().Gap(13).AlignItems(ui.Center).Children(func() {
				ui.Image(c, fileArt[kind]).Size(65, 65).Fit(ui.Contain)
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, t.Name).FontSize(17).FontWeight(640).SingleLine()
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons[stateSymbol(t.Status)]).ContentTransition(ui.ContentTransition{}).FontSize(14).TextColor(color).Tooltip(statusText(t.Status))
						size := taskSize(t)
						if size != "" {
							ui.Text(c, size).ContentTransition(ui.ContentTransition{Numeric: true}).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
						}
					})
				})
				ui.Icon(c, icons["chevron"]).FontSize(16).Rotate(-90).TextColor(p.muted)
			})
		})
		if head.Clicked() {
			v.detailID = t.ID
			c.Invalidate()
		}
		taskProgress(c, p, t, color)
		ui.Box(c).Grow(1)
		v.rowActions(c, t)
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

func taskProgress(c *ui.Context, p palette, t application.TaskDTO, color ui.Color) {
	active := t.Status == "active"

	section := ui.Column(c).Gap(6)
	fraction := section.Animate("completion", float32(t.Progress), 220*time.Millisecond)
	section.Children(func() {
		ui.Row(c).Height(18).AlignItems(ui.Center).Children(func() {
			metric, ink := "", p.muted
			if t.Error != "" {
				metric, ink = t.Error, color
			} else if active && t.Speed > 0 {
				metric = bytesText(t.Speed) + "/s"
			}
			ui.Text(c, metric).MinWidth(0).Grow(1).SingleLine().Tooltip(metric).FontSize(12).FontFeatures("tnum").TextColor(ink).ContentTransition(ui.ContentTransition{})
			if t.Total > 0 && t.Status != "complete" {
				ui.Textf(c, "%.0f%%", 100*t.Progress).FontSize(12).FontWeight(600).FontFeatures("tnum").ContentTransition(ui.ContentTransition{Numeric: true})
			}
		})
		track := ui.Box(c).Height(5).FillWidth().Radius(2.5).Background(p.border).Clip().Role(ui.RoleProgress).Label("下载进度")
		known := t.Total > 0 || t.Status == "complete"
		if known {
			track.Range(0, 1, t.Progress)
		} else {
			track.Description("文件大小未知")
		}
		// Only the visible track requests paint frames. Counters and layout
		// continue to update on actual engine observations.
		target := float32(0)
		if active && !c.Preferences().ReduceMotion && (!known || t.Speed > 0 && fraction > 0) {
			target = 1
		}
		strength := track.Animate("flow", target, 200*time.Millisecond)
		track.Draw(func(painter *ui.Painter, r ui.Rect) {
			fill := r
			if known {
				fill.W *= max(0, min(1, fraction))
				painter.Fill(fill, color, 2.5)
			}
			if strength <= 0 {
				if active && !known {
					// Three centered dots indicate an unknown length without
					// suggesting a measured fraction of the file.
					for i := range 3 {
						painter.Fill(ui.Rect{X: r.X + r.W/2 + float32(i-1)*9 - 2.5, Y: r.Y, W: 5, H: r.H}, color, 2.5)
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
			painter.Clip(fill, 2.5, func() {
				painter.FillGradient(ui.Rect{X: x, Y: r.Y, W: band, H: r.H}, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
					{Color: shine.Alpha(0), At: 0}, {Color: shine.Alpha(strength), At: .55}, {Color: shine.Alpha(0), At: 1},
				}}, 2.5)
			})
		})
	})
}

func (v *View) taskDetail(c *ui.Context, p palette, t application.TaskDTO) {
	ui.Column(c.Key("detail")).Grow(1).Gap(12).Transition(panelMotion).Children(func() {
		back := ui.Button(c, "返回任务").AlignSelf(ui.Start).Label("收起详情 " + t.Name).Height(33).Radius(16)
		if back.Clicked() || c.Shortcut(0, ui.KeyEscape) {
			v.detailID = ""
			c.Invalidate()
		}
		card := ui.Column(c.Key(t.ID)).Padding(22).Gap(15).Transition(cardMotion(t.ID))
		color := card.AnimateColor("status", taskColor(c, p, t.Status), ui.Animation{Duration: 230 * time.Millisecond})
		surface(c, card, p, color, 22, false).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Image(c, fileArt[fileIcon(t.Name)]).Size(60, 60).Fit(ui.Contain)
				ui.Text(c, t.Name).FontSize(22).FontWeight(660).Grow(1).MinWidth(0).SingleLine().Tooltip(t.Name)
				ui.Text(c, statusText(t.Status)).ContentTransition(ui.ContentTransition{}).FontSize(12).TextColor(color)
			})
			if size := taskSize(t); size != "" {
				ui.Text(c, size).ContentTransition(ui.ContentTransition{Numeric: true}).FontSize(12).TextColor(p.muted)
			}
			taskProgress(c, p, t, color)

			ui.Divider(c)
			detailField(c, p, "来源", t.URL)
			target := t.Path
			if target == "" {
				target = t.Directory
			}
			detailField(c, p, "位置", target)
			v.rowActions(c, t)
		})
	})
}

func detailField(c *ui.Context, p palette, label, value string) {
	ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Text(c, label).Width(44).FontSize(12).TextColor(p.muted)
		ui.Text(c, value).FontSize(12).Grow(1).MinWidth(0).SingleLine().Selectable().Tooltip(value)
	})
}
