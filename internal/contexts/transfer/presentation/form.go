package presentation

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (v *View) newDialog(c *ui.Context, p palette, reveal float32) {
	open := v.DialogOpen
	ui.DialogBase(c, &open, func(back, panel ui.Element) {
		back.Column().Justify(ui.End).AlignItems(ui.Center).Padding(0, 0, 12, sidebarWidth).Background(ui.Hex("#303443").Alpha(.23)).Transition(ui.ElementTransition{
			Duration: 150 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{},
		})
		scale := .78 + .22*reveal
		panel.TransformOrigin(.5, 1).Scale(scale, scale).Translate(0, 6*(1-reveal))
		surface(c, panel.Column().Width(dockWidth).Padding(20, 24).Gap(11), p, p.tint, 25, true).Shadow(0, 8, 30, 0, p.accent.Alpha(.13)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "新建下载").FontSize(21).FontWeight(660).Grow(1)
				if iconButton(c, "close", "关闭新建下载", v.Busy) {
					v.DialogOpen = false
				}
			})
			ui.Divider(c).Margin(2, 0)
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "下载链接").FontWeight(600).Grow(1)
				lines := 0
				for line := range strings.Lines(v.URLs) {
					if strings.TrimSpace(line) != "" {
						lines++
					}
				}
				if lines > 1 {
					ui.Textf(c, "已输入 %d 行", lines).FontSize(11).TextColor(p.muted)
				}
			})
			if ui.TextArea(c, &v.URLs).Height(115).Padding(10).Background(p.canvas).Placeholder("粘贴 HTTP 或 HTTPS 链接，每行一个").Label("下载链接").AutoFocus().Disabled(v.Busy).Changed() {
				v.FormError = ""
			}
			ui.Text(c, "保存到").FontWeight(600)
			ui.Row(c).Gap(8).Children(func() {
				if ui.TextInput(c, &v.Directory).Grow(1).MinWidth(0).Label("保存目录").Disabled(v.Busy).Changed() {
					v.FormError = ""
				}
				if ui.Button(c, "选择…").Disabled(v.Busy || v.Actions.PickDirectory == nil).Clicked() {
					v.Actions.PickDirectory(v.Directory, func(path string, err error) {
						if err != nil {
							v.FormError = err.Error()
						} else if path != "" {
							v.Directory = path
							v.FormError = ""
						}
					})
				}
			})
			if v.FormError != "" {
				ui.Text(c, v.FormError).MaxHeight(68).Tooltip(v.FormError).FontSize(12).TextColor(c.Theme().Danger)
			}
			ui.Divider(c).Margin(2, 0)
			ui.Row(c).Gap(8).Justify(ui.End).Margin(4, 0, 0).Children(func() {
				if ui.Button(c, "取消").Disabled(v.Busy).Clicked() {
					v.DialogOpen = false
				}
				if primaryAction(c, "开始下载", "download", v.Busy).Width(106).Clicked() {
					v.submit()
				}
			})
		})
	})
	if !open && !v.Busy {
		v.DialogOpen = false
	}
}
