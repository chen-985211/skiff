package presentation

import (
	"context"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation/i18n"
)

// Keep the paint layer alive through dismissal, but release its modal
// input scope immediately so the underlying trigger can reverse the spring.
func optionDialog(c *ui.Context, p palette, stage ui.Element, key string, open *bool, content func(ui.Element)) {
	reveal := stage.AnimateTo(key, amount(*open), panelSpring)
	if !*open && reveal == 0 {
		return
	}
	ui.Overlay(c, func() {
		back := ui.Box(c.Key(key)).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().
			Background(p.scrim.Alpha(.12 * reveal)).IgnoreInput(!*open).AccessHidden(!*open)
		if *open {
			back.Modal()
			if back.Clicked() || back.OverlayShortcut(0, ui.KeyEscape) {
				*open = false
				c.Invalidate()
			}
		}
		back.Children(func() {
			panel := ui.Box(c.Key("panel")).Role(ui.RoleDialog).Opacity(reveal).Translate(0, 8*(1-reveal))
			panel.Clicked() // Presses on the panel itself stay inside the modal.
			content(panel)
		})
	})
}

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
func (v *View) ftpLogin(c *ui.Context, p palette, stage ui.Element) {
	optionDialog(c, p, stage, "ftp", &v.ftpOpen, func(panel ui.Element) {
		panel.Width(320).Column().Padding(20).Gap(12).Radius(cornerSheet).Background(p.panel).Children(func() {
			ui.Text(c, tr(c, "ftp.signIn")).FontSize(18).FontWeight(600)
			optionField(c, p, tr(c, "ftp.user"), &v.ftpUser, false)
			optionField(c, p, tr(c, "ftp.password"), &v.ftpPassword, true)
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
				if secondaryAction(c, tr(c, "common.cancel")).Clicked() {
					v.ftpOpen = false
				}
				if primaryAction(c, tr(c, "ftp.continue"), "download", v.ftpUser == "" || v.Busy).Clicked() {
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

const preferenceWidth = 396
const preferenceInset = 20
const preferenceBorder = 1

func preferenceSheet(c *ui.Context, p palette, stage ui.Element, key, title, icon string, open *bool, body func()) {
	optionDialog(c, p, stage, key, open, func(panel ui.Element) {
		panel.Label(title).Width(preferenceWidth).Column().Padding(preferenceInset).Gap(16).
			Radius(cornerSheet).Background(p.panel).Border(preferenceBorder, p.border.Alpha(.5)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(9).Children(func() {
				ui.Box(c).Size(30, 30).Center().Radius(cornerSmall).Background(p.tint.Alpha(.38)).Children(func() {
					ui.Icon(c, icons[icon]).FontSize(17).TextColor(p.active).AccessHidden(true)
				})
				ui.Text(c, title).Grow(1).FontSize(18).FontWeight(600)
				close := ui.ButtonBase(c.Key("close")).Transition(controlMotion).Size(30, 30).Radius(cornerRemove).
					Center().FocusRing(true).Label(tr(c, "common.close") + title).Tooltip(tr(c, "common.close"))
				if close.Hovered() {
					close.Background(p.tint.Alpha(.45))
				}
				close.Children(func() { ui.Icon(c, icons["close"]).FontSize(15).TextColor(p.muted) })
				if close.Clicked() {
					*open = false
					c.Invalidate()
				}
			})
			body()
		})
	})
}

// Keep the unit visible when the user replaces the placeholder with a number.
func preferenceNumber(c *ui.Context, p palette, label, unit, placeholder, hint string, value *string) ui.Element {
	var field ui.Element
	ui.Column(c.Key(label)).FillWidth().Gap(4).Children(func() {
		field = numberWithUnit(c, p, label, unit, placeholder, value).Description(trf(c, "common.unitHint", i18n.Args{"unit": unit, "hint": hint}))
		ui.Text(c, hint).Margin(0, 0, 0, optionLabelWidth+optionGap).FontSize(12).TextColor(p.muted)
	})
	return field
}

func numberWithUnit(c *ui.Context, p palette, label, unit, placeholder string, value *string) ui.Element {
	var field ui.Element
	formRow(c, p, label, func() {
		ui.Row(c).Grow(1).MinWidth(0).Height(32).AlignItems(ui.Center).Children(func() {
			field = rowInput(c, p, label, placeholder, value, false).Padding(0, 50, 0, 9).Description(unit)
			ui.Box(c).Absolute().Right(10).Top(0).Bottom(0).Center().AccessHidden(true).IgnoreInput(true).Children(func() {
				ui.Text(c, unit).FontSize(12).TextColor(p.muted)
			})
		})
	})
	return field
}
