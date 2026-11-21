package presentation

import (
	"context"
	"errors"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"strconv"
	"strings"
)

func (v *View) openDisk(d application.DiskSettings) {
	v.cacheText, v.splitText = "", ""
	if d.CacheMiB != 0 || d.DisableCache {
		v.cacheText = strconv.Itoa(d.CacheMiB)
	}
	if d.SplitMiB != 0 {
		v.splitText = strconv.Itoa(d.SplitMiB)
	}
	v.allocation = d.Allocation
	v.diskErr = nil
	v.diskOpen = true
}
func optionalNumber(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.Atoi(strings.TrimSpace(value))
}
func (v *View) diskDialog(c *ui.Context, p palette, stage ui.Element) {
	preferenceSheet(c, p, stage, "disk", "性能与磁盘", "tray", &v.diskOpen, func() {
		if preferenceNumber(c, p, "磁盘缓存", "MiB", "自动", "填 0 关闭缓存，重启后生效", &v.cacheText).Changed() {
			v.diskErr = nil
		}
		if preferenceNumber(c, p, "最小分段", "MiB", "自动", "用于新建下载", &v.splitText).Changed() {
			v.diskErr = nil
		}
		formRow(c, p, "文件预分配", func() {
			label := map[string]string{"": "自动", "prealloc": "预分配", "none": "关闭"}[v.allocation]
			dropdown(c.Key("allocation"), p, "文件预分配 · "+label, label, v.Busy, func(m *ui.Menu) {
				for _, item := range [][2]string{{"自动", ""}, {"预分配", "prealloc"}, {"关闭", "none"}} {
					if m.Item(item[0]).Checked(v.allocation == item[1]).Chosen() {
						v.allocation = item[1]
						v.diskErr = nil
					}
				}
			}).Grow(1)
		})
		inlineError(c, v.diskErr)
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			if secondaryAction(c, "恢复默认").Clicked() {
				v.openDisk(application.DiskSettings{})
			}
			if primaryAction(c, "保存", "check", v.Busy).Clicked() {
				cache, e1 := optionalNumber(v.cacheText)
				split, e2 := optionalNumber(v.splitText)
				if e1 != nil || e2 != nil || v.splitText != "" && split == 0 {
					v.diskErr = errors.New("请输入有效的 MiB 数值")
					return
				}
				d := application.DiskSettings{CacheMiB: cache, SplitMiB: split, DisableCache: v.cacheText != "" && cache == 0, Allocation: v.allocation}
				v.run(func(ctx context.Context) error { return v.API.SetDiskSettings(ctx, d) }, func(err error) { v.diskErr = err; v.diskOpen = err != nil })
			}
		})
	})
}
