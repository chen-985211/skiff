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
	open                        bool
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
func (v *View) renderBTNetwork(c *ui.Context, p palette, stage ui.Element) {
	b := &v.btNetwork
	preferenceSheet(c, p, stage, "bt-network", "BT 网络", "network", &b.open, func() {
		ui.Scroll(c).FillWidth().Height(264).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				ui.Text(c, "Tracker 订阅源").FontSize(12).TextColor(p.muted)
				ui.TextAreaBase(c, &b.draft.Sources).Transition(controlMotion).Label("Tracker 订阅源").Placeholder("每行一个订阅地址").FillWidth().Height(58).Padding(7).Border(1, p.border).Radius(cornerSmall)
				for _, sub := range v.Snapshot.BTSubscriptions {
					msg := fmt.Sprintf("%d 个 Tracker", len(sub.Trackers))
					if sub.Error != "" {
						msg = sub.Error
					}
					ui.Text(c, sub.URL+" · "+msg).SingleLine().TruncateMiddle().Tooltip(sub.URL + " · " + msg).FontSize(11).TextColor(p.muted)
				}
				formRow(c, p, "节点发现", func() {
					dht := !b.draft.DisableDHT
					if ui.Checkbox(c, &dht, "DHT").Transition(controlMotion).Changed() {
						b.draft.DisableDHT = !dht
					}
					ui.Checkbox(c, &b.draft.LocalDiscovery, "局域网").Transition(controlMotion)
				})
				formRow(c, p, "监听端口", func() { rowInput(c, p, "监听端口", "自动", &b.port, false) })
				formRow(c, p, "节点上限", func() { rowInput(c, p, "每种节点上限", "自动", &b.peers, false) })
				ui.Checkbox(c, &b.draft.Seed.Enabled, "下载完成后继续做种").Transition(controlMotion)
				preferenceNumber(c, p, "目标分享率", "倍", "不限", "先达到任一条件即停止", &b.ratio)
				preferenceNumber(c, p, "做种时长", "分钟", "不限", "先达到任一条件即停止", &b.minutes)
				ui.Text(c, "端口和节点发现重启生效；订阅每 12 小时更新。").FontSize(11).TextColor(p.muted)
			})
		})
		inlineError(c, b.err)
		if primaryAction(c, "保存并更新", "check", v.Busy).AlignSelf(ui.End).Clicked() {
			port, e1 := optionalNumber(b.port)
			peers, e2 := optionalNumber(b.peers)
			if e1 != nil || e2 != nil {
				b.err = errors.New("请输入有效的端口和节点数")
				return
			}
			cfg := b.draft
			cfg.ListenPort, cfg.MaxPeers = port, peers
			var err error
			cfg.Seed.Minutes, err = optionalNumber(b.minutes)
			if err == nil && strings.TrimSpace(b.ratio) != "" {
				cfg.Seed.Ratio, err = strconv.ParseFloat(strings.TrimSpace(b.ratio), 64)
			} else {
				cfg.Seed.Ratio = 0
			}
			if err != nil {
				b.err = errors.New("请输入有效的做种时长和分享率")
				return
			}
			api := v.API.(btNetworkAPI)
			v.run(func(ctx context.Context) error {
				if err := api.SetBTSettings(ctx, cfg); err != nil {
					return err
				}
				return api.RefreshBTSubscriptions(ctx, true)
			}, func(err error) { b.err = err; b.open = err != nil })
		}
	})
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
