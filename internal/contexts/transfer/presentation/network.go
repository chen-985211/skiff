package presentation

import (
	"context"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation/i18n"
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
func (v *View) networkDialog(c *ui.Context, p palette, stage ui.Element) {
	preferenceSheet(c, p, stage, "network", tr(c, "network.proxyDNS"), "network", &v.networkOpen, func() {
		v.proxyOptions(c, p, &v.networkDraft, false, preferenceWidth-2*(preferenceInset+preferenceBorder))
		ui.Box(c).Role(ui.RoleSeparator).FillWidth().Height(1).Background(p.border.Alpha(.55))
		formRow(c, p, "DNS", func() {
			label := tr(c, "common.auto")
			if v.networkDraft.DNS == "system" {
				label = tr(c, "network.systemDNS")
			}
			dropdown(c.Key("dns"), p, "DNS · "+label, label, v.Busy, func(m *ui.Menu) {
				if m.Item(tr(c, "common.auto")).Checked(v.networkDraft.DNS == "").Chosen() {
					v.networkDraft.DNS = ""
				}
				if m.Item(tr(c, "network.systemDNS")).Checked(v.networkDraft.DNS == "system").Chosen() {
					v.networkDraft.DNS = "system"
				}
			}).Grow(1)
		})
		inlineError(c, v.networkErr)
		if primaryAction(c, tr(c, "common.save"), "check", v.Busy).AlignSelf(ui.End).Clicked() {
			value := normalizedNetwork(v.networkDraft)
			if err := value.Validate(); err != nil {
				v.networkErr = err
				return
			}
			v.run(func(ctx context.Context) error { return v.API.SetNetwork(ctx, value) }, func(err error) { v.networkErr = err; v.networkOpen = err != nil })
		}
	})
}

type proxyChoice struct{ mode, label string }

var proxyChoices = [...]proxyChoice{{"", "network.global"}, {"system", "common.system"}, {"direct", "network.direct"}, {"manual", "network.manual"}}

func (v *View) proxyOptions(c *ui.Context, p palette, n *application.NetworkSettings, task bool, width float32) {
	ui.Column(c.Key("proxy-options")).Width(width).Shrink(0).Children(func() {
		choices := proxyChoices[:]
		if !task {
			choices = choices[1:]
		}
		track := v.proxySelector(c, p, n, width, choices)
		manual := n.Mode == "manual"
		visible := track.AnimateTo("manual-fields", amount(manual), panelSpring)
		inherits := task && n.Mode == ""
		hint := track.AnimateTo("inheritance", amount(inherits), panelSpring)
		line := ui.Box(c.Key("inherited-proxy")).FillWidth().Height(26 * hint).Clip().Opacity(hint).AccessHidden(!inherits)
		if inherits || hint > 0 {
			label := tr(c, "common.system")
			switch v.Snapshot.Preferences.Network.Mode {
			case "direct":
				label = tr(c, "network.direct")
			case "manual":
				label = v.Snapshot.Preferences.Network.Proxy
			}
			line.Children(func() {
				ui.Text(c, trf(c, "network.currentGlobal", i18n.Args{"mode": label})).Absolute().Left(0).Right(0).Top(7).TruncateMiddle().Tooltip(label).SingleLine().FontSize(12).TextColor(p.muted)
			})
		}
		// Include the gap above the fields in the reveal. An outer layout gap
		// would appear in full on the very first frame of a zero-height editor.
		details := ui.Box(c.Key("proxy-credentials")).FillWidth().Height(117 * visible).Shrink(0).Clip().
			Opacity(visible).IgnoreInput(!manual).AccessHidden(!manual)
		if manual || visible > 0 {
			details.Children(func() {
				ui.Column(c).Absolute().Top(7).Left(0).Right(0).Gap(7).Children(func() {
					formRow(c, p, tr(c, "network.address"), func() {
						if rowInput(c, p, tr(c, "network.proxyURL"), "http://proxy:8080", &n.Proxy, false).Changed() {
							v.networkErr = nil
							v.FormError = nil
						}
					})
					formRow(c, p, tr(c, "network.auth"), func() {
						rowInput(c, p, tr(c, "network.user"), tr(c, "common.username"), &n.User, false)
						rowInput(c, p, tr(c, "network.password"), tr(c, "common.password"), &n.Password, true)
					})
					formRow(c, p, tr(c, "network.bypass"), func() { rowInput(c, p, tr(c, "network.bypass"), tr(c, "network.bypassHint"), &n.Bypass, false) })
				})
			})
		}
	})
}

func (v *View) proxySelector(c *ui.Context, p palette, n *application.NetworkSettings, width float32, choices []proxyChoice) ui.Element {
	index := 0
	for i, choice := range choices {
		if choice.mode == n.Mode {
			index = i
		}
	}
	parts := ui.SegmentedBase(c, &index, len(choices))
	cell := (width - 4) / float32(len(choices))
	track := parts.Track.Label(tr(c, "network.mode")).Width(width).Height(32).Shrink(0).Padding(2).Radius(cornerCompact).Background(p.haze.Alpha(.22))
	x := track.AnimateTo("selection", float32(index)*cell, selectionSpring)
	track.Draw(func(painter *ui.Painter, r ui.Rect) {
		painter.Fill(ui.Rect{X: r.X + 2 + x, Y: r.Y + 2, W: cell, H: 28}, p.panel, cornerSmall)
	})
	track.Children(func() {
		for i, choice := range choices {
			item := parts.Segment(i).ZIndex(1).Transition(controlMotion).Size(cell, 28).Label(tr(c, choice.label)).Radius(cornerSmall).FocusRing(true)
			item.Children(func() {
				ui.Text(c, tr(c, choice.label)).SingleLine().TextColor(c.Theme().Text).FontSize(12).FontWeight(560)
			})
			if item.Changed() {
				n.Mode = choice.mode
				v.networkErr = nil
				v.FormError = nil
				c.Invalidate()
			}
		}
	})
	return track
}
