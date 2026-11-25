package presentation

import (
	"context"
	"encoding/hex"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation/i18n"
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
	fileMessageKey string
	fileErr        error
	peerLimit      string
	open, pending  bool
	id             string
	data           application.BTDetails
	err            error
	next           time.Time
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
	{"common.connection", "common.connection", "network"}, {"Tracker", "Tracker", "list"}, {"bt.seeding", "bt.seeding", "upload"},
}

func (v *View) renderBTNetwork(c *ui.Context, p palette, stage ui.Element) {
	b := &v.btNetwork
	preferenceSheet(c, p, stage, "bt-network", tr(c, "bt.network"), "network", &b.open, func() {
		const width = preferenceWidth - 2*(preferenceInset+preferenceBorder)
		optionTabBar(c, p, tr(c, "bt.settingsTabs"), &b.tab, btNetworkTabs[:], width).Disabled(v.Busy)
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
			if primaryAction(c, tr(c, "bt.saveRefresh"), "check", v.Busy).Shrink(0).Clicked() {
				v.saveBTSettings()
			}
		})
	})
}

func (b *btNetworkPanel) connections(c *ui.Context, p palette) {
	formRow(c, p, tr(c, "bt.discovery"), func() {
		dht := !b.draft.DisableDHT
		if ui.Checkbox(c, &dht, "DHT").Transition(controlMotion).Changed() {
			b.draft.DisableDHT = !dht
		}
		ui.Checkbox(c, &b.draft.LocalDiscovery, tr(c, "bt.localNetwork")).Transition(controlMotion)
	})
	formRow(c, p, tr(c, "bt.listenPort"), func() { rowInput(c, p, tr(c, "bt.listenPort"), tr(c, "common.auto"), &b.port, false) })
	formRow(c, p, tr(c, "bt.peerLimit"), func() { rowInput(c, p, tr(c, "bt.perTorrentLimit"), tr(c, "common.auto"), &b.peers, false) })
	ui.Text(c, tr(c, "bt.requiresRestart")).FontSize(12).TextColor(p.muted)
}

const btSubscriptionRowHeight = 24

func (v *View) trackerSubscriptions(c *ui.Context, p palette) {
	b := &v.btNetwork
	ui.Row(c).FillWidth().Height(18).AlignItems(ui.Center).Children(func() {
		ui.Text(c, tr(c, "bt.subscriptionURLs")).Grow(1).FontSize(13).FontWeight(560)
		ui.Text(c, tr(c, "bt.refreshInterval")).FontSize(12).TextColor(p.muted)
	})
	// Give the editor three full lines. MyGo paints scrolling text through
	// its own padding, so the vertical insets belong to the surrounding box.
	frame := ui.Box(c).FillWidth().Height(68).Padding(6, 0).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.42))
	if frame.FocusWithin() {
		frame.Border(1, p.active)
	}
	frame.Children(func() {
		field := ui.TextAreaBase(c, &b.draft.Sources).Transition(controlMotion).Label(tr(c, "bt.trackerSubscriptions")).
			Placeholder(tr(c, "bt.subscriptionPerLine")).Fill().Clip().Padding(0, settingsScrollGutter, 0, 9).
			ScrollbarStyle(settingsScrollbar(c, p)).FontSize(13).FixedLineHeight(18)
		if b.showFirstSource {
			field.SetTextSelection(0, 0)
			b.showFirstSource = false
		}
	})
	// Only results scroll. The editor stays at the full form width, with the
	// thumb in a separate gutter beside the status column.
	ui.Scroll(c.Key("subscription-results")).ScrollbarStyle(settingsScrollbar(c, p)).Role(ui.RoleList).Label(tr(c, "bt.subscriptionStatus")).
		Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(0, settingsScrollGutter, 0, 0).Children(func() {
			for _, sub := range v.Snapshot.BTSubscriptions {
				ui.Row(c.Key(sub.URL)).Role(ui.RoleListItem).FillWidth().Height(btSubscriptionRowHeight).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, sub.URL).Grow(1).MinWidth(0).SingleLine().TruncateMiddle().Tooltip(sub.URL).FontSize(12).TextColor(p.muted)
					status, color := trf(c, "bt.trackerCount", i18n.Args{"count": len(sub.Trackers)}), p.muted
					var reason string
					if sub.Failure != "" {
						reason = errorText(c.Locale(), sub.Failure)
						status, color = tr(c, "bt.refreshFailed"), c.Theme().Danger
					} else if sub.Checked.IsZero() {
						status = tr(c, "bt.pendingRefresh")
					}
					ui.Text(c, status).SingleLine().FontSize(12).TextColor(color).ContentTransition(textChange).
						Description(reason).Tooltip(reason)
				})
			}
		})
	})
}

func (b *btNetworkPanel) seeding(c *ui.Context, p palette) {
	toggle := ui.Checkbox(c, &b.draft.Seed.Enabled, tr(c, "bt.seedCompleted")).Transition(controlMotion).Height(32)
	on := b.draft.Seed.Enabled
	reveal := toggle.AnimateTo("stop-conditions", amount(on), panelSpring)
	if !on && reveal == 0 {
		return
	}
	ui.Column(c.Key("seed-limits")).FillWidth().Gap(10).Opacity(reveal).Translate(0, -4*(1-reveal)).
		AccessHidden(!on).IgnoreInput(!on).Children(func() {
		numberWithUnit(c, p, tr(c, "bt.targetRatio"), tr(c, "bt.ratioUnit"), tr(c, "bt.noSeedingLimit"), &b.ratio)
		numberWithUnit(c, p, tr(c, "bt.seedTime"), tr(c, "bt.minutes"), tr(c, "bt.noSeedingLimit"), &b.minutes)
		ui.Text(c, tr(c, "bt.stopAtLimit")).FontSize(12).TextColor(p.muted)
	})
}

func (v *View) saveBTSettings() {
	b := &v.btNetwork
	cfg := b.draft
	port, e1 := optionalNumber(b.port)
	peers, e2 := optionalNumber(b.peers)
	if e1 != nil || e2 != nil {
		b.tab, b.err = btConnections, copyError("bt.invalidPeers", nil)
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
			b.tab, b.err = btSeeding, copyError("bt.invalidSeeding", nil)
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
	preferenceSheet(c, p, stage, "bt-inspector", tr(c, "bt.details"), "network", &b.open, func() {
		formRow(c, p, tr(c, "bt.peerLimit"), func() {
			rowInput(c, p, tr(c, "bt.torrentPeerLimit"), tr(c, "bt.inherit"), &b.peerLimit, false)
			if secondaryAction(c, tr(c, "common.apply")).Disabled(v.Busy).Clicked() {
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
		ui.Text(c, trf(c, "bt.swarmTotals", i18n.Args{"pieces": d.Pieces, "uploaded": bytesText(d.Uploaded)})).FontSize(13)
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
				ui.Text(c, trf(c, "bt.trackersHeading", i18n.Args{"count": len(d.Trackers)})).FontWeight(600)
				for _, tr := range d.Trackers {
					ui.Text(c, tr).SingleLine().TruncateMiddle().Tooltip(tr).FontSize(12)
				}
				ui.Text(c, trf(c, "bt.connectedPeers", i18n.Args{"count": len(d.Peers)})).FontWeight(600)
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
