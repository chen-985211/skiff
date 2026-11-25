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
	ui.Column(c.Key("torrent-create")).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			rowInput(c, p, tr(c, "bt.createSource"), tr(c, "bt.fileOrFolder"), &b.source, false).Disabled(v.Busy)
			dropdown(c.Key("pick-source"), p, tr(c, "bt.chooseSource"), tr(c, "common.choose"), v.Busy || v.Actions.PickSource == nil, func(m *ui.Menu) {
				for i, name := range []string{tr(c, "bt.pickFile"), tr(c, "bt.pickFolder")} {
					if m.Item(name).Chosen() {
						v.Actions.PickSource(i == 1, func(path string, err error) {
							if !b.open || !v.DialogOpen {
								return
							}
							if err != nil {
								v.FormError = err
							} else if path != "" {
								b.source = path
								v.FormError = nil
							}
						})
					}
				}
			})
		})
		ui.TextAreaBase(c, &b.trackers).Transition(controlMotion).Label(tr(c, "bt.createTrackers")).Placeholder(tr(c, "bt.trackerHint")).FillWidth().Height(56).Padding(8).Border(1, p.border).Radius(cornerSmall).Disabled(v.Busy)
	})
}
func (v *View) createFooter(c *ui.Context, p palette, width float32) {
	b := &v.bt.creator
	ui.Checkbox(c, &b.seed, tr(c, "bt.seedOriginal")).Transition(controlMotion).Grow(1).Disabled(v.Busy)
	if b.cancel != nil {
		if secondaryAction(c, tr(c, "bt.cancelCreate")).Width(width).Shrink(0).Clicked() {
			b.cancel()
		}
		return
	}
	if !primaryAction(c, tr(c, "bt.save"), "file", v.Busy || strings.TrimSpace(b.source) == "" || v.Actions.SaveTorrent == nil).Width(width).Shrink(0).Clicked() {
		return
	}
	source, trackers, seed := filepath.Clean(strings.TrimSpace(b.source)), strings.Fields(b.trackers), b.seed
	v.Actions.SaveTorrent(source+".torrent", func(path string, err error) {
		if !b.open || !v.DialogOpen || v.bt.tab != 1 {
			return
		}
		if err != nil {
			v.FormError = err
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
		v.FormError = nil
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
				v.FormError = err
				return
			}
			b.open = false
			b.saved = path
			b.source = ""
			b.trackers = ""
		})
	})
}
