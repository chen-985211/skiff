package presentation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

type torrentCreateAPI interface {
	CreateTorrent(context.Context, string, string, []string, bool) (application.Torrent, error)
}
type torrentCreator struct {
	open, seed              bool
	source, trackers, saved string
	cancel                  context.CancelFunc
}

func (v *View) createEditor(c *ui.Context, p palette) {
	b := &v.bt.creator
	ui.Column(c.Key("torrent-create")).Transition(pageFade).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			rowInput(c, p, "制种来源", "选择文件或文件夹", &b.source, false).Disabled(v.Busy)
			dropdown(c.Key("pick-source"), p, "选择制种来源", "选择", v.Busy || v.Actions.PickSource == nil, func(m *ui.Menu) {
				for i, name := range []string{"文件…", "文件夹…"} {
					if m.Item(name).Chosen() {
						v.Actions.PickSource(i == 1, func(path string, err error) {
							if !b.open || !v.DialogOpen {
								return
							}
							if err != nil {
								v.FormError = err.Error()
							} else if path != "" {
								b.source = path
								v.FormError = ""
							}
						})
					}
				}
			})
		})
		ui.TextAreaBase(c, &b.trackers).Transition(controlMotion).Label("制种 Tracker").Placeholder("Tracker 地址（可选，每行一个）").FillWidth().Height(56).Padding(8).Border(1, p.border).Radius(cornerSmall).Disabled(v.Busy)
	})
}
func (v *View) createFooter(c *ui.Context, p palette) {
	b := &v.bt.creator
	ui.Checkbox(c, &b.seed, "用原文件做种").Transition(controlMotion).Grow(1).Disabled(v.Busy)
	if b.cancel != nil {
		if secondaryAction(c, "取消生成").Clicked() {
			b.cancel()
		}
		return
	}
	if !primaryAction(c, "保存种子…", "file", v.Busy || strings.TrimSpace(b.source) == "" || v.Actions.SaveTorrent == nil).Clicked() {
		return
	}
	source, trackers, seed := filepath.Clean(strings.TrimSpace(b.source)), strings.Fields(b.trackers), b.seed
	v.Actions.SaveTorrent(source+".torrent", func(path string, err error) {
		if !b.open || !v.DialogOpen || v.bt.tab != 1 {
			return
		}
		if err != nil {
			v.FormError = err.Error()
			return
		}
		if path == "" {
			return
		}
		if filepath.Ext(path) == "" {
			path += ".torrent"
		}
		parent := v.Actions.Context
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithCancel(parent)
		b.cancel = cancel
		api := v.API.(torrentCreateAPI)
		v.FormError = ""
		v.run(func(context.Context) error {
			defer cancel()
			_, e := api.CreateTorrent(ctx, source, path, trackers, seed)
			return e
		}, func(err error) {
			b.cancel = nil
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				v.FormError = err.Error()
				return
			}
			b.open = false
			b.saved = path
			b.source = ""
			b.trackers = ""
		})
	})
}
