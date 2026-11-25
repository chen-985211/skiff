package presentation

import (
	"errors"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
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
	if err == nil {
		return ""
	}
	var message messageError
	if errors.As(err, &message) {
		return i18n.Format(locale, message.key, message.args)
	}
	key := transferErrorMessages[application.CodeOf(err)]
	if key == "" {
		key = transferErrorMessages[application.ErrUnexpected]
	}
	return i18n.Text(locale, key)
}
