package presentation

import (
	"context"
	"github.com/egoist/mygo/ui"
	"strings"
)

func (v *View) openNetwork(task bool) {
	v.networkTask = task
	v.networkDraft = v.Snapshot.Preferences.Network
	if task {
		v.networkDraft = v.taskNetwork
	}
	v.networkErr = nil
	v.networkOpen = true
}
func (v *View) networkDialog(c *ui.Context, p palette) {
	ui.DialogBase(c, &v.networkOpen, func(back, panel ui.Element) {
		panel.Width(350).Column().Padding(18).Gap(10).Radius(cornerSheet).Background(p.panel).Transition(panelMotion).Children(func() {
			title := "代理与 DNS"
			if v.networkTask {
				title = "单任务代理"
			}
			ui.Text(c, title).FontSize(18).FontWeight(600)
			n := &v.networkDraft
			mode := n.Mode
			labels := map[string]string{"system": "跟随系统", "direct": "直连", "manual": "手动代理", "": "跟随系统"}
			modes := []string{"system", "direct", "manual"}
			if v.networkTask {
				labels[""] = "使用全局设置"
				modes = append([]string{""}, modes...)
			} else if mode == "" {
				mode = "system"
			}
			ui.MenuButton(c, labels[mode], func(m *ui.Menu) {
				for _, key := range modes {
					if m.Item(labels[key]).Checked(mode == key).Chosen() {
						n.Mode = key
					}
				}
			}).Transition(controlMotion)
			_, h := c.Size()
			if n.Mode == "manual" {
				ui.Scroll(c.Key("manual-proxy")).Transition(pageFade).FillWidth().Height(min(234, h-240)).Children(func() {
					ui.Column(c).FillWidth().Gap(6).Children(func() {
						optionField(c, p, "HTTP 代理地址", &n.Proxy, false)
						optionField(c, p, "代理账号", &n.User, false)
						optionField(c, p, "代理密码", &n.Password, true)
						optionField(c, p, "绕过代理（域名或网段，逗号分隔）", &n.Bypass, false)
					})
				})
			}
			if !v.networkTask {
				label := "DNS · 自动"
				if n.DNS == "system" {
					label = "DNS · 系统解析"
				}
				ui.MenuButton(c, label, func(m *ui.Menu) {
					if m.Item("自动").Checked(n.DNS == "").Chosen() {
						n.DNS = ""
					}
					if m.Item("系统解析").Checked(n.DNS == "system").Chosen() {
						n.DNS = "system"
					}
				}).Transition(controlMotion)
			}
			inlineError(c, v.networkErr)
			if primaryAction(c, "保存", "check", v.Busy).AlignSelf(ui.End).Clicked() {
				value := *n
				value.Proxy = strings.TrimSpace(value.Proxy)
				if value.Mode != "manual" {
					value.Proxy, value.User, value.Password, value.Bypass = "", "", "", ""
				}
				if err := value.Validate(); err != nil {
					v.networkErr = err
					return
				}
				if v.networkTask {
					v.taskNetwork = value
					v.networkOpen = false
				} else {
					v.run(func(ctx context.Context) error { return v.API.SetNetwork(ctx, value) }, func(err error) { v.networkErr = err; v.networkOpen = err != nil })
				}
			}
		})
	})
}
