package presentation

import (
	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation/i18n"
)

func (v *View) languageSelector(c *ui.Context, p palette) {
	current := v.Actions.ChosenLocale()
	display := tr(c, "common.system")
	for _, locale := range i18n.Languages() {
		if current == locale {
			display = i18n.LanguageName(locale)
			break
		}
	}
	dropdown(c.Key("language"), p, tr(c, "language.title"), display, false, func(m *ui.Menu) {
		if m.Item(tr(c, "common.system")).Checked(current == "").Chosen() {
			v.Actions.SetLocale("")
		}
		for _, locale := range i18n.Languages() {
			if m.Item(i18n.LanguageName(locale)).Checked(current == locale).Chosen() {
				v.Actions.SetLocale(locale)
			}
		}
	})
}
