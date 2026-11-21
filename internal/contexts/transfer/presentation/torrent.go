package presentation

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"path"
	"skiff/internal/contexts/transfer/application"
	"sort"
	"strings"
	"time"
)

type torrentAPI interface {
	PrepareTorrent(context.Context, string) (application.Torrent, error)
	AddTorrent(context.Context, string, string, []int) (string, error)
}
type torrentEntry struct {
	path         string
	depth, index int
}
type torrentDraft struct {
	creator    torrentCreator
	tab, wait  int
	source     string
	meta       application.Torrent
	selected   []bool
	rows       []torrentEntry
	list       ui.ListState
	cancel     context.CancelFunc
	generation uint64
	deadline   time.Time
}

func (b *torrentDraft) clearPreview() {
	b.meta = application.Torrent{}
	b.rows = nil
	b.selected = nil
	b.list = ui.ListState{}
}

func (b *torrentDraft) stopLookup() {
	b.deadline = time.Time{}
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
		b.generation++
	}
}
func (v *View) sourceTabs(c *ui.Context, p palette, ink float32) {
	b := &v.bt
	nav := ui.SegmentedBase(c, &b.tab, 2)
	track := nav.Track.Absolute().Left(0).Top(0).Label("下载类型").Size(128, 32).Padding(2).Radius(cornerCompact).Background(p.haze.Alpha(.23)).Opacity(editorOpacity(1 - ink)).IgnoreInput(v.advanced).AccessHidden(v.advanced)
	x := track.AnimateTo("source", float32(b.tab)*62, selectionSpring)
	track.Draw(func(painter *ui.Painter, r ui.Rect) {
		painter.Fill(ui.Rect{X: r.X + 2 + x, Y: r.Y + 2, W: 62, H: 28}, p.panel, cornerSmall)
	})
	track.Children(func() {
		for i, label := range []string{"链接", "BT"} {
			item := nav.Segment(i).Transition(controlMotion).Size(62, 28).FocusRing(true).Disabled(v.Busy).Label(label)
			item.Children(func() { ui.Text(c, label).SingleLine().FontSize(12).FontWeight(560) })
			if item.Changed() {
				b.stopLookup()
				v.FormError = ""
				c.Invalidate()
			}
		}
	})
}
func (v *View) lookupTorrent(source string) {
	api, ok := v.API.(torrentAPI)
	if !ok || !v.DialogOpen || v.bt.tab != 1 {
		return
	}
	b := &v.bt
	b.stopLookup()
	b.generation++
	seq := b.generation
	b.source = source
	b.clearPreview()
	v.FormError = ""
	parent := v.Actions.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	b.cancel = cancel
	go func() {
		defer cancel()
		meta, err := api.PrepareTorrent(ctx, source)
		v.dispatch(func() {
			if seq != b.generation {
				return
			}
			b.cancel = nil
			if err != nil {
				v.FormError = err.Error()
				return
			}
			b.meta = meta
			b.selected = make([]bool, len(meta.Files))
			b.rows = nil
			b.list = ui.ListState{}
			type entry struct {
				p string
				i int
			}
			files := make([]entry, len(meta.Files))
			for i, f := range meta.Files {
				files[i] = entry{f.Path, i}
				b.selected[i] = true
			}
			sort.Slice(files, func(i, j int) bool { return files[i].p < files[j].p })
			folders := map[string]bool{}
			for _, f := range files {
				parts := strings.Split(f.p, "/")
				for k := 1; k < len(parts); k++ {
					dir := strings.Join(parts[:k], "/")
					if !folders[dir] {
						folders[dir] = true
						b.rows = append(b.rows, torrentEntry{dir, k - 1, -1})
					}
				}
				b.rows = append(b.rows, torrentEntry{f.p, len(parts) - 1, f.i})
			}
			if b.wait > 0 && strings.HasPrefix(source, "magnet:") {
				b.deadline = time.Now().Add(time.Duration(b.wait) * time.Second)
			}
		})
	}()
}
func (v *View) torrentTools(c *ui.Context, p palette) {
	tools := ui.Box(c.Key("torrent-tools")).Size(composeToolWidth, formHeaderHeight)
	ink := tools.AnimateTo("creator", amount(v.bt.creator.open), colorFade)
	tools.Children(func() {
		for i, render := range []func(*ui.Context, palette){v.torrentOptions, v.returnToTorrent} {
			active := (i == 1) == v.bt.creator.open
			weight := pageWeight(i, ink)
			if !active && weight == 0 {
				continue
			}
			tool := ui.Box(c.Key(i)).Absolute().Right(0).Top(0).Bottom(0).
				Opacity(editorOpacity(weight)).IgnoreInput(!active).AccessHidden(!active)
			tool.Children(func() { render(c, p) })
		}
	})
}

func (v *View) returnToTorrent(c *ui.Context, p palette) {
	if secondaryAction(c, "返回下载").Disabled(v.Busy).Clicked() {
		v.bt.creator.open = false
		v.FormError = ""
		c.Invalidate()
	}
}

func (v *View) torrentOptions(c *ui.Context, p palette) {
	b := &v.bt
	dropdown(c.Key("bt-options"), p, "BT 选项", "BT 选项", v.Busy, func(m *ui.Menu) {
		if _, ok := v.API.(torrentCreateAPI); ok {
			if m.Item("创建种子…").Chosen() {
				b.stopLookup()
				b.creator.open = true
				b.creator.saved = ""
				v.FormError = ""
			}
			m.Separator()
		}
		for _, s := range []int{0, 30, 60, 120} {
			name := "手动选文件"
			if s > 0 {
				name = fmt.Sprintf("%d 秒后下载全部", s)
			}
			if m.Item(name).Checked(b.wait == s).Chosen() {
				b.wait = s
				b.deadline = time.Time{}
				if s > 0 && b.meta.Hash != "" && strings.HasPrefix(b.source, "magnet:") {
					b.deadline = time.Now().Add(time.Duration(s) * time.Second)
				}
			}
		}
	}).Width(composeToolWidth)
}

func (v *View) torrentEditor(c *ui.Context, p palette) {
	viewport := ui.Column(c.Key("torrent-editor")).FillWidth().MinHeight(sourceMinHeight)
	ink := viewport.AnimateTo("creator", amount(v.bt.creator.open), colorFade)
	sourceEditors(c, viewport, v.bt.creator.open, ink,
		func() { v.torrentDownloadEditor(c, p) }, func() { v.createEditor(c, p) })
}

func (v *View) torrentDownloadEditor(c *ui.Context, p palette) {
	b := &v.bt
	ui.Column(c.Key("torrent-download")).FillWidth().Gap(8).Children(func() {
		if b.creator.saved != "" {
			ui.Text(c, "已保存 "+b.creator.saved).Transition(pageFade).SingleLine().TruncateMiddle().Tooltip(b.creator.saved).FontSize(12).TextColor(p.muted)
		}
		if b.meta.Hash == "" {
			magnet := ui.TextAreaBase(c.Key("magnet"), &b.source).Transition(controlMotion).
				Label("磁力链接").Placeholder("粘贴磁力链接").PlaceholderColor(p.muted).FillWidth().Height(64).
				Padding(12, 14).FontSize(13).FixedLineHeight(20).Radius(cornerField).Disabled(b.cancel != nil || v.Busy)
			styleSourceField(c, magnet, p)
			ui.Row(c.Key("torrent-source-actions")).Transition(pageFade).FillWidth().Height(32).AlignItems(ui.Center).Gap(10).Children(func() {
				pick := ui.ButtonBase(c.Key("open-torrent")).Transition(controlMotion).Label("打开种子").Tooltip("打开本地 .torrent 文件").
					Row().Height(32).PaddingX(10).AlignItems(ui.Center).Gap(6).Radius(cornerSmall).FocusRing(true).
					Disabled(v.Actions.PickTorrent == nil || b.cancel != nil || v.Busy)
				if pick.Hovered() || pick.Pressed() {
					pick.Background(p.panel.Alpha(.5))
				}
				pick.Children(func() {
					ui.Icon(c, icons["folder"]).Size(15, 15).TextColor(p.muted)
					ui.Text(c, "打开种子…").SingleLine().FontSize(12).FontWeight(560).TextColor(p.muted)
				})
				if pick.Clicked() {
					v.Actions.PickTorrent(func(file string, err error) {
						if err != nil {
							v.FormError = err.Error()
						} else if file != "" {
							v.lookupTorrent(file)
						}
					})
				}
				if b.cancel != nil {
					ui.Text(c, "正在获取文件列表…").Transition(pageFade).SingleLine().FontSize(12).TextColor(p.muted)
				}
			})
			return
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, b.meta.Name).Grow(1).SingleLine().TruncateMiddle().FontWeight(600).Tooltip(b.meta.Name)
			if secondaryAction(c, "更换种子").Disabled(v.Busy).Clicked() {
				b.stopLookup()
				b.clearPreview()
				b.source = ""
				c.Invalidate()
			}
		})
		if b.meta.Hash == "" {
			return
		}
		ui.List(c.Key("torrent-tree"), &b.list, len(b.rows), func(i int) {
			r := b.rows[i]
			ui.Row(c.Key(r.path)).FillWidth().Height(28).PaddingX(float32(min(r.depth, 8))*12 + 4).AlignItems(ui.Center).Gap(6).Children(func() {
				if r.index < 0 {
					ui.Icon(c, icons["folder"]).FontSize(14).TextColor(p.muted)
					ui.Text(c, path.Base(r.path)).Transition(pageFade).FontSize(12).SingleLine()
					return
				}
				if ui.Checkbox(c, &b.selected[r.index], path.Base(r.path)).Label(r.path).Transition(controlMotion).Grow(1).MinWidth(0).Disabled(v.Busy).Changed() {
					b.deadline = time.Time{}
				}
				ui.Text(c, bytesText(b.meta.Files[r.index].Size)).FontSize(11).TextColor(p.muted)
			})
		}).FillWidth().Height(120).Label("种子文件树")
		if !b.deadline.IsZero() && v.DialogOpen && b.tab == 1 && !b.creator.open {
			left := time.Until(b.deadline)
			if left <= 0 {
				b.deadline = time.Time{}
				for i := range b.selected {
					b.selected[i] = true
				}
				v.submitTorrent()
			} else {
				c.After(min(time.Second, left))
				ui.Textf(c, "%d 秒后下载全部文件", int(left.Seconds())+1).Transition(pageFade).FontSize(12).TextColor(p.muted)
			}
		}
	})
}
func (v *View) submitTorrent() {
	b := &v.bt
	if b.cancel != nil {
		b.stopLookup()
		return
	}
	if b.meta.Hash == "" {
		if strings.TrimSpace(b.source) == "" {
			v.FormError = "请打开种子或粘贴磁力链接"
			return
		}
		v.lookupTorrent(strings.TrimSpace(b.source))
		return
	}
	var indexes []int
	for i, sel := range b.selected {
		if sel {
			indexes = append(indexes, i+1)
		}
	}
	if len(indexes) == 0 {
		v.FormError = "请至少选择一个文件"
		return
	}
	dir, hash := v.Directory, b.meta.Hash
	b.deadline = time.Time{}
	api := v.API.(torrentAPI)
	v.run(func(ctx context.Context) error { _, err := api.AddTorrent(ctx, hash, dir, indexes); return err }, func(err error) {
		if err != nil {
			v.FormError = err.Error()
			return
		}
		v.DialogOpen = false
		v.bt = torrentDraft{wait: b.wait}
		v.FormError = ""
	})
}
