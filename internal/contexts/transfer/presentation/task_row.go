package presentation

import (
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

func (v *View) rowActions(c *ui.Context, t application.TaskDTO) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(5).Children(func() {
		if t.CanPause && iconButton(c, "pause", "暂停下载", v.Busy) {
			v.command(t.ID, application.Pause)
		}
		if t.CanResume && iconButton(c, "play", "继续下载", v.Busy) {
			v.command(t.ID, application.Resume)
		}
		if t.CanRetry && primaryAction(c, "重试下载", "retry", v.Busy).Width(106).Height(30).Clicked() {
			v.command(t.ID, application.Retry)
		}
		if t.CanRestart && ui.Button(c, "从头下载").FontSize(11).Disabled(v.Busy).Clicked() {
			v.RestartID, v.RestartOpen = t.ID, true
		}
		ui.Box(c).Grow(1)
		if iconButton(c, "folder", "在文件夹中显示", t.Path == "" || v.Busy) {
			if v.Actions.Reveal != nil {
				v.Actions.Reveal(t.Path)
			}
		}
		if iconButton(c, "close", "移除任务（保留文件）", v.Busy) {
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
		return p.accent
	case "complete":
		return p.green
	case "error":
		return c.Theme().Danger.Mix(c.Theme().Text, .22)
	default:
		return p.muted
	}
}

// Cool reflections distinguish file types without borrowing the status colors.
func fileTint(p palette, kind string) ui.Color {
	switch kind {
	case "media":
		return p.tint.Mix(ui.Hex("#99b9da"), .56)
	case "audio":
		return p.tint.Mix(ui.Hex("#c7adc9"), .55)
	case "archive":
		return p.tint.Mix(ui.Hex("#a8bfcb"), .55)
	default:
		return p.tint.Mix(p.warm, .68)
	}
}

func (v *View) taskCard(c *ui.Context, p palette, t application.TaskDTO) {
	color := taskColor(c, p, t.Status)
	card := ui.Column(c.Key(t.ID)).Fill().Padding(15, 17, 12).Gap(8).Transition(cardMotion(t.ID))
	kind := fileIcon(t.Name)
	plate := surface(c, card, p, p.tint, 20, false)
	if !c.Preferences().HighContrast && !c.Preferences().ReduceTransparency {
		reflected := fileTint(p, kind)
		if c.Theme().Dark {
			reflected = reflected.Mix(p.panel, .70)
		}
		plate.LinearGradient(ui.LinearGradient{Angle: 152, Stops: []ui.GradientStop{
			{Color: reflected.Alpha(.72), At: 0}, {Color: p.panel.Alpha(.90), At: .42},
			{Color: p.panel.Alpha(.82), At: .73}, {Color: reflected.Mix(p.panel, .52).Alpha(.78), At: 1},
		}})
	}
	plate.Children(func() {
		head := ui.ButtonBase(c).Label("查看详情 " + t.Name).Tooltip(t.Name + " · " + statusText(t.Status)).Height(71).FillWidth().Radius(13)
		if head.Hovered() {
			head.Background(p.panel.Alpha(.58))
		}
		head.Children(func() {
			ui.Row(c).Fill().Gap(13).AlignItems(ui.Center).Children(func() {
				ui.Image(c, fileArt[fileIcon(t.Name)]).Size(65, 65).Fit(ui.Contain)
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, t.Name).FontSize(17).FontWeight(640).SingleLine()
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons[stateSymbol(t.Status)]).FontSize(14).TextColor(color).Tooltip(statusText(t.Status))
						size := taskSize(t)
						if size != "" {
							ui.Text(c, size).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
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
		if t.Error != "" {
			ui.Text(c, t.Error).FontSize(12).TextColor(color).SingleLine().Tooltip(t.Error)
		}
		ui.Box(c).Grow(1)
		ui.Box(c).Height(1).FillWidth().Background(p.border.Alpha(.5))
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
	if t.Total <= 0 && !active {
		return
	}
	section := ui.Column(c).Gap(6)
	fraction := section.Animate("completion", float32(t.Progress), 220*time.Millisecond)
	section.Children(func() {
		if t.Status != "complete" && (t.Total > 0 || t.Speed > 0) {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				if active && t.Speed > 0 {
					ui.Text(c, bytesText(t.Speed)+"/s").FontSize(12).FontFeatures("tnum").TextColor(p.muted)
				}
				ui.Box(c).Grow(1)
				if t.Total > 0 {
					ui.Textf(c, "%.0f%%", 100*t.Progress).FontSize(12).FontWeight(600).FontFeatures("tnum")
				}
			})
		}
		track := ui.Box(c).Height(5).FillWidth().Radius(2.5).Background(p.border).Clip().Role(ui.RoleProgress).Label("下载进度")
		known := t.Total > 0
		if known {
			track.Range(0, 1, t.Progress)
		} else {
			track.Description("文件大小未知")
		}
		// Only the visible track requests paint frames. Counters and layout
		// continue to update on actual engine observations.
		flowing := active && !c.Preferences().ReduceMotion && (!known || t.Speed > 0 && fraction > 0)
		track.Draw(func(painter *ui.Painter, r ui.Rect) {
			fill := r
			if known {
				fill.W *= max(0, min(1, fraction))
				painter.Fill(fill, color, 2.5)
			}
			if !active {
				return
			}
			if !flowing {
				if !known {
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
					{Color: shine.Alpha(0), At: 0}, {Color: shine, At: .55}, {Color: shine.Alpha(0), At: 1},
				}}, 2.5)
			})
		})
	})
}

func (v *View) taskDetail(c *ui.Context, p palette, t application.TaskDTO) {
	ui.Column(c.Key("detail")).Grow(1).Gap(12).Children(func() {
		back := ui.Button(c, "返回任务").AlignSelf(ui.Start).Label("收起详情 " + t.Name).Height(33).Radius(16)
		if back.Clicked() || c.Shortcut(0, ui.KeyEscape) {
			v.detailID = ""
			c.Invalidate()
		}
		color := taskColor(c, p, t.Status)
		card := ui.Column(c.Key(t.ID)).Padding(22).Gap(15).Transition(cardMotion(t.ID))
		surface(c, card, p, color, 22, false).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Image(c, fileArt[fileIcon(t.Name)]).Size(60, 60).Fit(ui.Contain)
				ui.Text(c, t.Name).FontSize(22).FontWeight(660).Grow(1).MinWidth(0).SingleLine().Tooltip(t.Name)
				ui.Text(c, statusText(t.Status)).FontSize(12).TextColor(color)
			})
			if size := taskSize(t); size != "" {
				ui.Text(c, size).FontSize(12).TextColor(p.muted)
			}
			taskProgress(c, p, t, color)
			if t.Error != "" {
				ui.Text(c, t.Error).FontSize(13).MaxHeight(58).Tooltip(t.Error).TextColor(color)
			}
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
