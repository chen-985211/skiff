package transfer_test

import (
	"testing"

	"skiff/internal/contexts/transfer/presentation/i18n"
)

func TestDownloadTranslationsUseNamedValues(t *testing.T) {
	for _, test := range []struct{ locale, title, summary string }{
		{"zh-CN", "新建下载", "新增 1 · 重复 2 · 无效 3"},
		{"en-GB", "New Download", "Added 1 · Duplicates 2 · Invalid 3"},
		{"", "新建下载", "新增 1 · 重复 2 · 无效 3"},
	} {
		if got := i18n.Text(test.locale, "download.new"); got != test.title {
			t.Errorf("%s title = %q, want %q", test.locale, got, test.title)
		}
		if got := i18n.Format(test.locale, "batch.summary", i18n.Args{"invalid": 3, "added": 1, "duplicates": 2}); got != test.summary {
			t.Errorf("%s summary = %q, want %q", test.locale, got, test.summary)
		}
	}
}
