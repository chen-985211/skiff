package presentation

import (
	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation/i18n"
)

func tr(c *ui.Context, key string) string {
	return i18n.Text(c.Locale(), key)
}

func trf(c *ui.Context, key string, args i18n.Args) string {
	return i18n.Format(c.Locale(), key, args)
}
