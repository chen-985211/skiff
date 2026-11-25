package presentation

import (
	"errors"
	"strings"

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
	return e.text(i18n.Source)
}

func (e messageError) text(locale string) string {
	if e.key == "" {
		return ""
	}
	if len(e.args) == 0 {
		return i18n.Text(locale, e.key)
	}
	args := make(i18n.Args, len(e.args))
	for name, value := range e.args {
		if nested, ok := value.(error); ok {
			value = errorText(locale, nested)
		}
		args[name] = value
	}
	return i18n.Format(locale, e.key, args)
}

func copyError(key string, args i18n.Args) messageError {
	return messageError{key, args}
}

func errorText(locale string, err error) string {
	if err == nil {
		return ""
	}
	if batch, ok := err.(*application.BatchAddError); ok {
		var messages []string
		seen := make(map[string]bool)
		for _, failure := range batch.Unwrap() {
			message := errorText(locale, failure)
			if !seen[message] {
				messages = append(messages, message)
				seen[message] = true
			}
		}
		return strings.Join(messages, "\n")
	}
	var message messageError
	if errors.As(err, &message) {
		return message.text(locale)
	}
	code := application.CodeOf(err)
	var seed *application.TorrentSeedError
	if code == application.ErrTorrentSeed && errors.As(err, &seed) && seed.Cause != nil {
		return i18n.Format(locale, "error.torrentSeedCause", i18n.Args{"reason": errorText(locale, seed.Cause)})
	}
	var named application.NamedError
	if errors.As(err, &named) && named.Code == code {
		args := i18n.Args{"name": named.Name}
		switch code {
		case application.ErrInvalidTracker:
			return i18n.Format(locale, "error.invalidTrackerLine", i18n.Args{"line": named.Name})
		case application.ErrManagedHeader:
			return i18n.Format(locale, "error.managedHeaderNamed", args)
		case application.ErrTorrentFileMissing:
			return i18n.Format(locale, "error.torrentFileMissingNamed", args)
		case application.ErrTorrentFileMismatch:
			return i18n.Format(locale, "error.torrentFileMismatchNamed", args)
		case application.ErrTorrentFileChanged:
			return i18n.Format(locale, "error.torrentFileChangedNamed", args)
		case application.ErrTorrentDestinationChanged:
			return i18n.Format(locale, "error.torrentDestinationChangedNamed", args)
		case application.ErrTorrentSourceChanged:
			return i18n.Format(locale, "error.torrentSourceChangedNamed", args)
		}
	}
	key := transferErrorMessages[code]
	if key == "" {
		key = transferErrorMessages[application.ErrUnexpected]
	}
	return i18n.Text(locale, key)
}
