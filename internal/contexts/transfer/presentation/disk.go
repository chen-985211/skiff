package presentation

import (
	"context"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation/i18n"
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
	preferenceSheet(c, p, stage, "disk", tr(c, "disk.title"), "tray", &v.diskOpen, func() {
		if preferenceNumber(c, p, tr(c, "disk.cache"), "MiB", tr(c, "common.auto"), tr(c, "disk.cacheHint"), &v.cacheText).Changed() {
			v.diskErr = nil
		}
		if preferenceNumber(c, p, tr(c, "disk.minimumSegment"), "MiB", tr(c, "common.auto"), tr(c, "disk.newDownloads"), &v.splitText).Changed() {
			v.diskErr = nil
		}
		formRow(c, p, tr(c, "disk.allocation"), func() {
			label := map[string]string{"": tr(c, "common.auto"), "prealloc": tr(c, "disk.preallocate"), "none": tr(c, "disk.off")}[v.allocation]
			dropdown(c.Key("allocation"), p, trf(c, "disk.currentAllocation", i18n.Args{"mode": label}), label, v.Busy, func(m *ui.Menu) {
				for _, item := range [][2]string{{tr(c, "common.auto"), ""}, {tr(c, "disk.preallocate"), "prealloc"}, {tr(c, "disk.off"), "none"}} {
					if m.Item(item[0]).Checked(v.allocation == item[1]).Chosen() {
						v.allocation = item[1]
						v.diskErr = nil
					}
				}
			}).Grow(1)
		})
		inlineError(c, v.diskErr)
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			if secondaryAction(c, tr(c, "disk.reset")).Clicked() {
				v.openDisk(application.DiskSettings{})
			}
			if primaryAction(c, tr(c, "common.save"), "check", v.Busy).Clicked() {
				cache, e1 := optionalNumber(v.cacheText)
				split, e2 := optionalNumber(v.splitText)
				if e1 != nil || e2 != nil || v.splitText != "" && split == 0 {
					v.diskErr = copyError("disk.invalidMiB", nil)
					return
				}
				d := application.DiskSettings{CacheMiB: cache, SplitMiB: split, DisableCache: v.cacheText != "" && cache == 0, Allocation: v.allocation}
				v.run(func(ctx context.Context) error { return v.API.SetDiskSettings(ctx, d) }, func(err error) { v.diskErr = err; v.diskOpen = err != nil })
			}
		})
	})
}
