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

type messageError struct {
	key  string
	args i18n.Args
}

func (e messageError) Error() string {
	return i18n.Format(i18n.Source, e.key, e.args)
}

func copyError(key string, args i18n.Args) error {
	return messageError{key, args}
}

func errorText(locale string, err error) string {
	if message, ok := err.(messageError); ok {
		return i18n.Format(locale, message.key, message.args)
	}
	return err.Error()
}
