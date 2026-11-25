package presentation

import (
	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation/i18n"
)

func (v *View) languageSelector(c *ui.Context, p palette) {
	preference := v.Actions.ChosenLocale()
	current, display := "", tr(c, "common.system")
	if preference != "" {
		current = i18n.MatchLanguage(preference, c.Locale())
		if current == "" {
			display = i18n.LanguageName(preference)
		} else {
			display = i18n.LanguageName(current)
		}
	}
	dropdown(c.Key("language"), p, tr(c, "language.title"), display, false, func(m *ui.Menu) {
		if m.Item(tr(c, "common.system")).Checked(preference == "").Chosen() {
			v.Actions.SetLocale("")
		}
		for _, locale := range i18n.Languages() {
			if m.Item(i18n.LanguageName(locale)).Checked(current == locale).Chosen() {
				v.Actions.SetLocale(locale)
			}
		}
	})
}
