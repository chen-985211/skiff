package i18n

import (
	"embed"
	"io/fs"

	native "github.com/egoist/mygo/i18n"
)

const Source = "zh-CN"

type Args map[string]any

//go:embed catalogs/*.json
var files embed.FS

var messages = loadCatalog()

func loadCatalog() *native.Catalog {
	dir, err := fs.Sub(files, "catalogs")
	if err != nil {
		panic(err)
	}
	catalog, err := native.NewCatalog(dir, Source)
	if err != nil {
		panic(err)
	}
	native.Register(catalog)
	return catalog
}

func Languages() []string {
	return messages.Languages()
}

func LanguageName(locale string) string {
	return native.Autonym(locale)
}

func Text(locale, key string) string {
	return Format(locale, key, nil)
}

func Format(locale, key string, args Args) string {
	return messages.Format(locale, key, args)
}
