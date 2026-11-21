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
			b.fileMessage = "正在处理文件…"
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
					run(func(ctx context.Context) error { return api.UseTorrentFiles(ctx, id, path, move, false) }, "文件已校验，位置已更新")
				}
			})
		}
		dropdown(c.Key("bt-file-actions"), p, "BT 文件操作", "文件操作", v.Busy, func(m *ui.Menu) {
			if m.Item("校验文件").Chosen() {
				run(func(ctx context.Context) error { return api.VerifyTorrentFiles(ctx, id) }, "全部分片校验通过")
			}
			if v.Actions.PickDirectory != nil {
				if m.Item("重新定位到已有文件…").Chosen() {
					choose(false)
				}
				if m.Item("移动文件到…").Chosen() {
					choose(true)
				}
			}
			if m.Item("使用已有文件做种").Chosen() {
				run(func(ctx context.Context) error { return api.UseTorrentFiles(ctx, id, task.Directory, false, true) }, "文件已校验，已提交做种")
			}
		})
		if b.fileMessage != "" {
			ui.Text(c, b.fileMessage).Transition(pageFade).FontSize(12).TextColor(p.muted)
		}
		inlineError(c, b.fileErr)
		return
	}
}
