package presentation

import (
	"context"
	"errors"
	"github.com/egoist/mygo/ui"
	"strconv"
)

func rateLabel(rate int64) string {
	switch {
	case rate <= 0:
		return "不限速"
	case rate%(1<<20) == 0:
		return strconv.FormatInt(rate>>20, 10) + " MiB/s"
	case rate%1024 == 0:
		return strconv.FormatInt(rate>>10, 10) + " KiB/s"
	default:
		return strconv.FormatInt(rate, 10) + " B/s"
	}
}

func rateScope(id string) string {
	if id != "" {
		return "任务限速"
	}
	return "全局限速"
}

func (v *View) speedSelector(c *ui.Context, p palette, id string, current int64) {
	label, display := rateScope(id), rateLabel(current)
	title, style := display, fieldDropdown
	if id != "" {
		title, style = label+" · "+display, cardDropdown
	}
	dropdown(c.Key(label), p, style, label+" · "+display, title, !v.Ready || v.Busy, func(m *ui.Menu) {
		for _, rate := range []int64{0, 512 << 10, 1 << 20, 2 << 20, 5 << 20, 10 << 20} {
			if m.Item(rateLabel(rate)).Checked(rate == current).Chosen() {
				v.saveRate(id, rate)
			}
		}
		m.Separator()
		if m.Item("自定义…").Chosen() {
			v.rateID = id
			v.rateText = strconv.FormatInt(current>>10, 10)
			v.rateErr = nil
			v.rateOpen = true
		}
	})
}
func (v *View) saveRate(id string, rate int64) {
	v.run(func(ctx context.Context) error { return v.API.SetDownloadLimit(ctx, id, rate) }, func(err error) {
		v.rateID = id
		v.rateText = strconv.FormatInt(rate>>10, 10)
		v.rateErr = err
		v.rateOpen = err != nil
	})
}
func (v *View) rateDialog(c *ui.Context, p palette, stage ui.Element) {
	preferenceSheet(c, p, stage, "rate", rateScope(v.rateID), "clock", &v.rateOpen, func() {
		if preferenceNumber(c, p, "下载上限", "KiB/s", "0", "填 0 表示不限速", &v.rateText).Changed() {
			v.rateErr = nil
		}
		inlineError(c, v.rateErr)
		if primaryAction(c, "应用", "check", v.Busy).AlignSelf(ui.End).Clicked() {
			n, err := strconv.ParseUint(v.rateText, 10, 53)
			if err != nil {
				v.rateErr = errors.New("请输入非负整数")
				return
			}
			v.saveRate(v.rateID, int64(n)<<10)
		}
	})
}
