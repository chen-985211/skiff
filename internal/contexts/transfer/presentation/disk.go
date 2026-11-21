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
	optionDialog(c, p, stage, "disk", &v.diskOpen, func(panel ui.Element) {
		panel.Width(340).Column().Padding(18).Gap(10).Radius(cornerSheet).Background(p.panel).Children(func() {
			ui.Text(c, "性能与磁盘").FontSize(18).FontWeight(600)
			optionField(c, p, "磁盘缓存（MiB，重启生效）", &v.cacheText, false)
			optionField(c, p, "最小分段（MiB，新任务生效）", &v.splitText, false)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "文件预分配").Grow(1)
				label := map[string]string{"": "自动", "prealloc": "预分配", "none": "关闭"}[v.allocation]
				ui.MenuButton(c, label, func(m *ui.Menu) {
					for _, item := range [][2]string{{"自动", ""}, {"预分配", "prealloc"}, {"关闭", "none"}} {
						if m.Item(item[0]).Checked(v.allocation == item[1]).Chosen() {
							v.allocation = item[1]
						}
					}
				}).Transition(controlMotion)
			})
			inlineError(c, v.diskErr)
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
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
	})
}
