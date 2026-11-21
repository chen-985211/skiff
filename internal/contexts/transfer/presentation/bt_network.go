package presentation

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"strconv"
	"strings"
	"time"
)

type btNetworkAPI interface {
	SetTorrentPeerLimit(context.Context, string, int) error
	SetBTSettings(context.Context, application.BTSettings) error
	RefreshBTSubscriptions(context.Context, bool) error
	TorrentDetails(context.Context, string) (application.BTDetails, error)
}
type btNetworkPanel struct {
	tab                         int
	open, showFirstSource       bool
	draft                       application.BTSettings
	port, peers, minutes, ratio string
	err                         error
}
type btInspector struct {
	fileMessage   string
	fileErr       error
	peerLimit     string
	open, pending bool
	id            string
	data          application.BTDetails
	err           error
	next          time.Time
}

func (v *View) openBTNetwork() {
	b := &v.btNetwork
	b.tab = btConnections
	b.showFirstSource = true
	b.draft = v.Snapshot.Preferences.BT
	b.minutes, b.ratio = "", ""
	if b.draft.Seed.Minutes > 0 {
		b.minutes = strconv.Itoa(b.draft.Seed.Minutes)
	}
	if b.draft.Seed.Ratio > 0 {
		b.ratio = strconv.FormatFloat(b.draft.Seed.Ratio, 'f', -1, 64)
	}
	b.port = ""
	b.peers = ""
	if b.draft.ListenPort != 0 {
		b.port = strconv.Itoa(b.draft.ListenPort)
	}
	if b.draft.MaxPeers != 0 {
		b.peers = strconv.Itoa(b.draft.MaxPeers)
	}
	b.err = nil
	b.open = true
}

const (
	btConnections = iota
	btTrackers
	btSeeding
)

var btNetworkTabs = [...]optionTabItem{
	{"连接", "连接", "network"}, {"Tracker", "Tracker", "list"}, {"做种", "做种", "upload"},
}

func (v *View) renderBTNetwork(c *ui.Context, p palette, stage ui.Element) {
	b := &v.btNetwork
	preferenceSheet(c, p, stage, "bt-network", "BT 网络", "network", &b.open, func() {
		const width = preferenceWidth - 2*(preferenceInset+preferenceBorder)
		optionTabBar(c, p, "BT 设置类别", &b.tab, btNetworkTabs[:], width).Disabled(v.Busy)
		// Size the stable page for the address editor and three complete status rows.
		const pageHeight = 18 + 68 + 2*10 + 3*btSubscriptionRowHeight
		pages := ui.Box(c.Key("bt-network-pages")).FillWidth().Height(pageHeight)
		pages.Children(func() {
			for i := range btNetworkTabs {
				active := i == b.tab
				weight := pages.AnimateTo(i, amount(active), colorFade)
				if !active && weight == 0 {
					continue
				}
				ui.Column(c.Key(i)).Absolute().Left(0).Top(0).Right(0).Bottom(0).Gap(10).
					Opacity(editorOpacity(weight)).IgnoreInput(!active).AccessHidden(!active).Disabled(v.Busy).Children(func() {
					switch i {
					case btConnections:
						b.connections(c, p)
					case btTrackers:
						v.trackerSubscriptions(c, p)
					case btSeeding:
						b.seeding(c, p)
					}
				})
			}
		})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { inlineError(c, b.err) })
			if primaryAction(c, "保存并更新", "check", v.Busy).Shrink(0).Clicked() {
				v.saveBTSettings()
			}
		})
	})
}

func (b *btNetworkPanel) connections(c *ui.Context, p palette) {
	formRow(c, p, "节点发现", func() {
		dht := !b.draft.DisableDHT
		if ui.Checkbox(c, &dht, "DHT").Transition(controlMotion).Changed() {
			b.draft.DisableDHT = !dht
		}
		ui.Checkbox(c, &b.draft.LocalDiscovery, "局域网").Transition(controlMotion)
	})
	formRow(c, p, "监听端口", func() { rowInput(c, p, "监听端口", "自动", &b.port, false) })
	formRow(c, p, "节点上限", func() { rowInput(c, p, "每种节点上限", "自动", &b.peers, false) })
	ui.Text(c, "更改端口或节点发现后，重启 Skiff 生效。").FontSize(12).TextColor(p.muted)
}

const btSubscriptionRowHeight = 24

func (v *View) trackerSubscriptions(c *ui.Context, p palette) {
	b := &v.btNetwork
	ui.Row(c).FillWidth().Height(18).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "订阅地址").Grow(1).FontSize(13).FontWeight(560)
		ui.Text(c, "每 12 小时更新").FontSize(12).TextColor(p.muted)
	})
	// Give the editor three full lines. MyGo paints scrolling text through
	// its own padding, so the vertical insets belong to the surrounding box.
	frame := ui.Box(c).FillWidth().Height(68).Padding(6, 0).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.42))
	if frame.FocusWithin() {
		frame.Border(1, p.active)
	}
	frame.Children(func() {
		field := ui.TextAreaBase(c, &b.draft.Sources).Transition(controlMotion).Label("Tracker 订阅源").
			Placeholder("每行一个订阅地址").Fill().Clip().Padding(0, settingsScrollGutter, 0, 9).
			ScrollbarStyle(settingsScrollbar(c, p)).FontSize(13).FixedLineHeight(18)
		if b.showFirstSource {
			field.SetTextSelection(0, 0)
			b.showFirstSource = false
		}
	})
	// Only results scroll. The editor stays at the full form width, with the
	// thumb in a separate gutter beside the status column.
	ui.Scroll(c.Key("subscription-results")).ScrollbarStyle(settingsScrollbar(c, p)).Role(ui.RoleList).Label("订阅状态").
		Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(0, settingsScrollGutter, 0, 0).Children(func() {
			for _, sub := range v.Snapshot.BTSubscriptions {
				ui.Row(c.Key(sub.URL)).Role(ui.RoleListItem).FillWidth().Height(btSubscriptionRowHeight).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, sub.URL).Grow(1).MinWidth(0).SingleLine().TruncateMiddle().Tooltip(sub.URL).FontSize(12).TextColor(p.muted)
					status, color := fmt.Sprintf("%d 个 Tracker", len(sub.Trackers)), p.muted
					if sub.Error != "" {
						status, color = "更新失败", c.Theme().Danger
					} else if sub.Checked.IsZero() {
						status = "待更新"
					}
					ui.Text(c, status).SingleLine().FontSize(12).TextColor(color).ContentTransition(textChange).Description(sub.Error).Tooltip(sub.Error)
				})
			}
		})
	})
}

func (b *btNetworkPanel) seeding(c *ui.Context, p palette) {
	toggle := ui.Checkbox(c, &b.draft.Seed.Enabled, "下载完成后继续做种").Transition(controlMotion).Height(32)
	on := b.draft.Seed.Enabled
	reveal := toggle.AnimateTo("stop-conditions", amount(on), panelSpring)
	if !on && reveal == 0 {
		return
	}
	ui.Column(c.Key("seed-limits")).FillWidth().Gap(10).Opacity(reveal).Translate(0, -4*(1-reveal)).
		AccessHidden(!on).IgnoreInput(!on).Children(func() {
		numberWithUnit(c, p, "目标分享率", "倍", "不限", &b.ratio)
		numberWithUnit(c, p, "做种时长", "分钟", "不限", &b.minutes)
		ui.Text(c, "先达到任一条件时停止做种。").FontSize(12).TextColor(p.muted)
	})
}

func (v *View) saveBTSettings() {
	b := &v.btNetwork
	cfg := b.draft
	port, e1 := optionalNumber(b.port)
	peers, e2 := optionalNumber(b.peers)
	if e1 != nil || e2 != nil {
		b.tab, b.err = btConnections, errors.New("请输入有效的端口和节点数")
		return
	}
	cfg.ListenPort, cfg.MaxPeers = port, peers
	if cfg.Seed.Enabled {
		minutes, err := optionalNumber(b.minutes)
		ratio := 0.0
		if err == nil && strings.TrimSpace(b.ratio) != "" {
			ratio, err = strconv.ParseFloat(strings.TrimSpace(b.ratio), 64)
		}
		if err != nil {
			b.tab, b.err = btSeeding, errors.New("请输入有效的时长和分享率")
			return
		}
		cfg.Seed.Minutes, cfg.Seed.Ratio = minutes, ratio
	}
	if err := cfg.Validate(); err != nil {
		b.err = err
		switch {
		case (application.BTSettings{Seed: cfg.Seed}).Validate() != nil:
			b.tab = btSeeding
		case (application.BTSettings{ListenPort: port, MaxPeers: peers}).Validate() != nil:
			b.tab = btConnections
		default:
			b.tab = btTrackers
		}
		return
	}
	b.err = nil
	api := v.API.(btNetworkAPI)
	v.run(func(ctx context.Context) error {
		if err := api.SetBTSettings(ctx, cfg); err != nil {
			return err
		}
		return api.RefreshBTSubscriptions(ctx, true)
	}, func(err error) { b.err = err; b.open = err != nil })
}

func (v *View) showBTDetails(id string) {
	v.btInfo = btInspector{open: true, id: id}
	for _, task := range v.Snapshot.Tasks {
		if task.ID == id && task.PeerLimit > 0 {
			v.btInfo.peerLimit = strconv.Itoa(task.PeerLimit)
		}
	}
}
func (v *View) renderBTDetails(c *ui.Context, p palette, stage ui.Element) {
	b := &v.btInfo
	if b.open {
		if !b.pending && !time.Now().Before(b.next) {
			id := b.id
			b.pending = true
			api := v.API.(btNetworkAPI)
			parent := v.Actions.Context
			if parent == nil {
				parent = context.Background()
			}
			go func() {
				ctx, cancel := context.WithTimeout(parent, 4*time.Second)
				defer cancel()
				data, err := api.TorrentDetails(ctx, id)
				v.dispatch(func() {
					if b.id == id && b.open {
						b.err = err
						if err == nil {
							b.data = data
						}
						b.pending = false
						b.next = time.Now().Add(2 * time.Second)
					}
				})
			}()
		}
		c.After(2 * time.Second)
	}
	preferenceSheet(c, p, stage, "bt-inspector", "BT 详情", "network", &b.open, func() {
		formRow(c, p, "节点上限", func() {
			rowInput(c, p, "此种子节点上限", "使用全局设置", &b.peerLimit, false)
			if secondaryAction(c, "应用").Disabled(v.Busy).Clicked() {
				n, err := optionalNumber(b.peerLimit)
				if err != nil {
					b.err = err
					return
				}
				id := b.id
				v.run(func(ctx context.Context) error { return v.API.(btNetworkAPI).SetTorrentPeerLimit(ctx, id, n) }, func(err error) { b.err = err })
			}
		})
		d := b.data
		bits, _ := hex.DecodeString(d.Bitfield)
		ui.Textf(c, "%d 个分片  ·  已上传 %s", d.Pieces, bytesText(d.Uploaded)).FontSize(13)
		ui.Box(c).FillWidth().Height(8).Radius(cornerSmall).Clip().Background(p.tint).Draw(func(painter *ui.Painter, r ui.Rect) {
			if d.Pieces <= 0 {
				return
			}
			for i := 0; i < min(len(bits)*8, d.Pieces); {
				if bits[i/8]&(128>>uint(i%8)) == 0 {
					i++
					continue
				}
				start := i
				for i < min(len(bits)*8, d.Pieces) && bits[i/8]&(128>>uint(i%8)) != 0 {
					i++
				}
				painter.Fill(ui.Rect{X: r.X + r.W*float32(start)/float32(d.Pieces), Y: r.Y, W: r.W * float32(i-start) / float32(d.Pieces), H: r.H}, p.active, 0)
			}
		})
		ui.Scroll(c).Height(168).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Textf(c, "Tracker · %d", len(d.Trackers)).FontWeight(600)
				for _, tr := range d.Trackers {
					ui.Text(c, tr).SingleLine().TruncateMiddle().Tooltip(tr).FontSize(12)
				}
				ui.Textf(c, "已连接节点 · %d", len(d.Peers)).FontWeight(600)
				for _, peer := range d.Peers {
					down, _ := strconv.ParseInt(peer.Download, 10, 64)
					up, _ := strconv.ParseInt(peer.Upload, 10, 64)
					ui.Text(c, strings.Join([]string{peer.Address, "↓ " + bytesText(down) + "/s", "↑ " + bytesText(up) + "/s"}, "   ")).FontSize(12)
				}
			})
		})
		v.btFileActions(c, p)
		inlineError(c, b.err)
	})
}
