package presentation

import (
	"context"
	"errors"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
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
		run := func(action func(context.Context) error, successKey string) {
			b.fileMessageKey = "bt.fileWorking"
			b.fileErr = nil
			v.run(action, func(err error) {
				if b.id == id {
					b.fileMessageKey = ""
					b.fileErr = err
					if err == nil {
						b.fileMessageKey = successKey
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
					run(func(ctx context.Context) error { return api.UseTorrentFiles(ctx, id, path, move, false) }, "bt.locationUpdated")
				}
			})
		}
		dropdown(c.Key("bt-file-actions"), p, tr(c, "bt.fileActions"), tr(c, "bt.files"), v.Busy, func(m *ui.Menu) {
			if m.Item(tr(c, "bt.verifyFiles")).Chosen() {
				run(func(ctx context.Context) error { return api.VerifyTorrentFiles(ctx, id) }, "bt.verified")
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
				run(func(ctx context.Context) error { return api.UseTorrentFiles(ctx, id, task.Directory, false, true) }, "bt.verifiedSeeding")
			}
		})
		if b.fileMessageKey != "" {
			ui.Text(c, tr(c, b.fileMessageKey)).Transition(pageFade).ContentTransition(textChange).FontSize(12).TextColor(p.muted)
		}
		btFileError(c, p, b.fileErr)
		return
	}
}

func btFileError(c *ui.Context, p palette, err error) {
	var cleanup *application.TorrentMoveCleanupError
	if !errors.As(err, &cleanup) {
		if message := errorText(c.Locale(), err); message != "" {
			ui.Text(c.Key("bt-file-error"), message).FillWidth().MaxLines(3).Tooltip(message).
				Transition(pageFade).ContentTransition(textChange).FontSize(12).TextColor(c.Theme().Danger)
		}
		return
	}
	const rowHeight = 22
	group := ui.Column(c.Key(cleanup)).FillWidth().Gap(6).Transition(pageFade)
	messageKey := ui.Local(group, "warning", func() string {
		if errors.Is(cleanup, application.ErrTorrentDestinationChanged) {
			return "error.torrentMoveCleanupVerify"
		}
		return "error.torrentMoveCleanup"
	})
	group.Children(func() {
		ui.Text(c, tr(c, *messageKey)).FontSize(12).TextColor(c.Theme().Danger)
		ui.List(c.Key("original-files"), nil, len(cleanup.Originals), func(i int) {
			original := cleanup.Originals[i]
			reasonKey := "bt.moveRemoveFailed"
			switch application.CodeOf(original.Cause) {
			case application.ErrTorrentDestinationChanged:
				reasonKey = "bt.moveDestinationChanged"
			case application.ErrTorrentFileChanged:
				reasonKey = "bt.moveOriginalChanged"
			}
			ui.Row(c).MinHeight(rowHeight).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, original.Name).Grow(1).MinWidth(0).Padding(3, 4).FontSize(12).TextColor(p.muted).
					SingleLine().TruncateMiddle().Selectable().Tooltip(original.Name)
				ui.Text(c, tr(c, reasonKey)).SingleLine().FontSize(12).TextColor(p.muted)
			})
		}).Height(float32(min(3, len(cleanup.Originals))*rowHeight)).FillWidth().Padding(0, settingsScrollGutter, 0, 0).
			Label(tr(c, "bt.moveOriginals")).Focusable().FocusRing(true).Radius(cornerSmall).
			ScrollbarStyle(settingsScrollbar(c, p)).Transition(panelMotion)
	})
}
