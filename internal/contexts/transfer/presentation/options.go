package presentation

import (
	"context"
	"github.com/egoist/mygo/ui"
)

// Keep labels, editing surfaces and focus feedback consistent across option dialogs.
func optionField(c *ui.Context, p palette, label string, value *string, secret bool) {
	ui.Column(c.Key(label)).FillWidth().Gap(4).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(p.muted)
		field := ui.TextInputBase(c, value).Transition(controlMotion).Label(label).FillWidth().Height(30).PaddingX(8).Radius(cornerSmall).Border(1, p.border).Background(p.panel.Alpha(.5)).FocusRing(true)
		if secret {
			field.Password()
		}
	})
}
func (v *View) ftpLogin(c *ui.Context, p palette) {
	ui.DialogBase(c, &v.ftpOpen, func(back, panel ui.Element) {
		panel.Width(320).Column().Padding(20).Gap(12).Radius(cornerSheet).Background(p.panel).Transition(panelMotion).Children(func() {
			ui.Text(c, "登录 FTP").FontSize(18).FontWeight(600)
			optionField(c, p, "FTP 账号", &v.ftpUser, false)
			optionField(c, p, "FTP 密码", &v.ftpPassword, true)
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
				if secondaryAction(c, "取消").Clicked() {
					v.ftpOpen = false
				}
				if primaryAction(c, "登录并继续", "download", v.ftpUser == "" || v.Busy).Clicked() {
					id, user, password := v.ftpID, v.ftpUser, v.ftpPassword
					v.ftpOpen = false
					v.run(func(ctx context.Context) error { return v.API.RetryFTP(ctx, id, user, password) }, func(err error) { v.setTaskError(id, err) })
				}
			})
		})
	})
	if !v.ftpOpen {
		v.ftpPassword = ""
	}
}
