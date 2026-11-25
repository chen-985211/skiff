package presentation

import (
	"context"
	"github.com/egoist/mygo/ui"
)

type btFileAPI interface {
	VerifyTorrentFiles(context.Context, string) error
	UseTorrentFiles(context.Context, string, string, bool, bool) error
}

func (v *View) btFileActions(c *ui.Context, p palette) {
	api, ok := v.API.(btFileAPI)
	if !ok {
		return
	}
	b := &v.btInfo
	for _, task := range v.Snapshot.Tasks {
		if task.ID != b.id {
			continue
		}
		id := task.ID
		run := func(action func(context.Context) error, msg string) {
			b.fileMessage = tr(c, "bt.fileWorking")
			b.fileErr = nil
			v.run(action, func(err error) {
				if b.id == id {
					b.fileMessage = ""
					b.fileErr = err
					if err == nil {
						b.fileMessage = msg
					}
				}
			})
		}
		choose := func(move bool) {
			v.Actions.PickDirectory(task.Directory, func(path string, err error) {
				if b.id != id || !b.open {
					return
				}
				if err != nil {
					b.fileErr = err
					return
				}
				if path != "" {
					run(func(ctx context.Context) error { return api.UseTorrentFiles(ctx, id, path, move, false) }, tr(c, "bt.locationUpdated"))
				}
			})
		}
		dropdown(c.Key("bt-file-actions"), p, tr(c, "bt.fileActions"), tr(c, "bt.files"), v.Busy, func(m *ui.Menu) {
			if m.Item(tr(c, "bt.verifyFiles")).Chosen() {
				run(func(ctx context.Context) error { return api.VerifyTorrentFiles(ctx, id) }, tr(c, "bt.verified"))
			}
			if v.Actions.PickDirectory != nil {
				if m.Item(tr(c, "bt.relocate")).Chosen() {
					choose(false)
				}
				if m.Item(tr(c, "bt.move")).Chosen() {
					choose(true)
				}
			}
			if m.Item(tr(c, "bt.seedExisting")).Chosen() {
				run(func(ctx context.Context) error { return api.UseTorrentFiles(ctx, id, task.Directory, false, true) }, tr(c, "bt.verifiedSeeding"))
			}
		})
		if b.fileMessage != "" {
			ui.Text(c, b.fileMessage).Transition(pageFade).FontSize(12).TextColor(p.muted)
		}
		inlineError(c, b.fileErr)
		return
	}
}
