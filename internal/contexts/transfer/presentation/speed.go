package presentation

import (
	"context"
	"errors"
	"github.com/egoist/mygo/ui"
	"strconv"
)

func (v *View) speedSelector(c *ui.Context, id string, current int64) {
	label := "全局限速"
	if id != "" {
		label = "任务限速"
	}
	display := "不限速"
	if current > 0 {
		display = bytesText(current) + "/s"
	}
	ui.MenuButton(c, label+" · "+display, func(m *ui.Menu) {
		for _, rate := range []int64{0, 512 << 10, 1 << 20, 2 << 20, 5 << 20, 10 << 20} {
			name := "不限速"
			if rate > 0 {
				name = bytesText(rate) + "/s"
			}
			if m.Item(name).Checked(rate == current).Chosen() {
				v.saveRate(id, rate)
			}
		}
		if m.Item("自定义…").Chosen() {
			v.rateID = id
			v.rateText = strconv.FormatInt(current>>10, 10)
			v.rateErr = nil
			v.rateOpen = true
		}
	}).Transition(controlMotion).Disabled(!v.Ready || v.Busy)
}
func (v *View) saveRate(id string, rate int64) {
	v.run(func(ctx context.Context) error { return v.API.SetDownloadLimit(ctx, id, rate) }, func(err error) {
		v.rateID = id
		v.rateText = strconv.FormatInt(rate>>10, 10)
		v.rateErr = err
		v.rateOpen = err != nil
	})
}
func (v *View) rateDialog(c *ui.Context, p palette) {
	ui.DialogBase(c, &v.rateOpen, func(back, panel ui.Element) {
		panel.Width(300).Column().Padding(18).Gap(10).Radius(cornerSheet).Background(p.panel).Transition(panelMotion).Children(func() {
			optionField(c, p, "下载上限（KiB/s，0 为不限速）", &v.rateText, false)
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
	})
}
