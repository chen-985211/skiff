package presentation

import (
	"context"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"strings"
)

func (v *View) openNetwork() {
	v.networkDraft = v.Snapshot.Preferences.Network
	v.networkErr = nil
	v.networkOpen = true
}
func normalizedNetwork(n application.NetworkSettings) application.NetworkSettings {
	n.Proxy = strings.TrimSpace(n.Proxy)
	if n.Mode != "manual" {
		n.Proxy, n.User, n.Password, n.Bypass = "", "", "", ""
	}
	return n
}
func (v *View) networkDialog(c *ui.Context, p palette) {
	ui.DialogBase(c, &v.networkOpen, func(back, panel ui.Element) {
		panel.Width(396).Column().Padding(18).Gap(12).Radius(cornerSheet).Background(p.panel).Transition(panelMotion).Children(func() {
			ui.Text(c, "代理与 DNS").FontSize(18).FontWeight(600)
			v.proxyOptions(c, p, &v.networkDraft, false, 360)
			formRow(c, p, "DNS", func() {
				label := "自动"
				if v.networkDraft.DNS == "system" {
					label = "系统解析"
				}
				ui.MenuButton(c, label, func(m *ui.Menu) {
					if m.Item("自动").Checked(v.networkDraft.DNS == "").Chosen() {
						v.networkDraft.DNS = ""
					}
					if m.Item("系统解析").Checked(v.networkDraft.DNS == "system").Chosen() {
						v.networkDraft.DNS = "system"
					}
				}).Grow(1).Transition(controlMotion)
			})
			inlineError(c, v.networkErr)
			if primaryAction(c, "保存", "check", v.Busy).AlignSelf(ui.End).Clicked() {
				value := normalizedNetwork(v.networkDraft)
				if err := value.Validate(); err != nil {
					v.networkErr = err
					return
				}
				v.run(func(ctx context.Context) error { return v.API.SetNetwork(ctx, value) }, func(err error) { v.networkErr = err; v.networkOpen = err != nil })
			}
		})
	})
}

type proxyChoice struct{ mode, label string }

var proxyChoices = [...]proxyChoice{{"", "全局设置"}, {"system", "跟随系统"}, {"direct", "直连"}, {"manual", "手动代理"}}

func (v *View) proxyOptions(c *ui.Context, p palette, n *application.NetworkSettings, task bool, width float32) {
	choices := proxyChoices[:]
	if !task {
		choices = choices[1:]
	}
	index := 0
	for i, choice := range choices {
		if choice.mode == n.Mode {
			index = i
		}
	}
	parts := ui.SegmentedBase(c, &index, len(choices))
	cell := (width - 4) / float32(len(choices))
	track := parts.Track.Label("代理模式").Width(width).Height(32).Shrink(0).Padding(2).Radius(cornerCompact).Background(p.haze.Alpha(.22))
	x := track.AnimateTo("selection", float32(index)*cell, selectionSpring)
	track.Draw(func(painter *ui.Painter, r ui.Rect) {
		painter.Fill(ui.Rect{X: r.X + 2 + x, Y: r.Y + 2, W: cell, H: 28}, p.panel, cornerSmall)
	})
	track.Children(func() {
		for i, choice := range choices {
			item := parts.Segment(i).ZIndex(1).Transition(controlMotion).Size(cell, 28).Label(choice.label).Radius(cornerSmall).FocusRing(true)
			item.Children(func() { ui.Text(c, choice.label).SingleLine().TextColor(c.Theme().Text).FontSize(12).FontWeight(560) })
			if item.Changed() {
				n.Mode = choice.mode
				v.networkErr = nil
				v.FormError = ""
				c.Invalidate()
			}
		}
	})
	manual := n.Mode == "manual"
	visible := track.AnimateTo("manual-fields", amount(manual), panelSpring)
	// Four fields, with the optional login on one row. Keep the visible page and
	// its action anchored as the detail area opens or reverses.
	if manual || visible > 0 {
		ui.Column(c.Key("proxy-credentials")).Transition(controlMotion).FillWidth().Shrink(0).Height(117*visible).Clip().Opacity(visible).IgnoreInput(!manual).AccessHidden(!manual).Disabled(!manual).Gap(7).Padding(3, 0, 0).Children(func() {
			formRow(c, p, "地址", func() {
				if rowInput(c, p, "HTTP 代理地址", "http://proxy:8080", &n.Proxy, false).Changed() {
					v.networkErr = nil
					v.FormError = ""
				}
			})
			formRow(c, p, "身份验证", func() {
				rowInput(c, p, "代理账号", "账号", &n.User, false)
				rowInput(c, p, "代理密码", "密码", &n.Password, true)
			})
			formRow(c, p, "绕过代理", func() { rowInput(c, p, "绕过代理", "域名或网段，逗号分隔", &n.Bypass, false) })
		})
	}
	if task && n.Mode == "" {
		label := "跟随系统"
		switch v.Snapshot.Preferences.Network.Mode {
		case "direct":
			label = "直连"
		case "manual":
			label = v.Snapshot.Preferences.Network.Proxy
		}
		ui.Text(c.Key("inherited-proxy"), "当前全局设置 · "+label).Transition(pageFade).ContentTransition(textChange).SingleLine().FontSize(12).TextColor(p.muted)
	}
}
