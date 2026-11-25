package transfer_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
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

func TestDownloadWorkspaceChangesLanguageWithoutResettingSearch(t *testing.T) {
	view := presentation.New(nil, presentation.Actions{})
	view.Ready = true
	view.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "archive.zip", Status: "paused"}}
	view.Query = "archive"
	scene := interactionScene(view, 800, 560)
	if !scene.HasText("全部任务") {
		t.Fatalf("Chinese navigation missing: %v", scene.Texts())
	}
	scene.SetLocale("en")
	scene.Frame()
	for _, label := range []string{"All Tasks", "New Download", "archive.zip", "1 result"} {
		if !scene.HasText(label) {
			t.Errorf("English workspace missing %q: %v", label, scene.Texts())
		}
	}
	if view.Query != "archive" {
		t.Fatalf("search was reset to %q", view.Query)
	}
}

func TestLanguageDropdownUsesSharedSelectionAndPreservesSettings(t *testing.T) {
	if got := i18n.LanguageName(i18n.Source); got != "简体中文" {
		t.Fatalf("Chinese locale name = %q", got)
	}
	var chosen string
	view := presentation.New(nil, presentation.Actions{
		ChosenLocale: func() string { return chosen },
		SetLocale:    func(locale string) { chosen = locale },
	})
	scene := interactionScene(view, 1020, 680)
	if err := scene.Click("设置"); err != nil {
		t.Fatal(err)
	}
	clickLanguageButton(t, scene, "界面语言")
	if err := scene.Click(i18n.LanguageName("en")); err != nil {
		t.Fatal(err)
	}
	if chosen != "en" || view.Busy {
		t.Fatalf("choosing English left locale %q and busy=%t", chosen, view.Busy)
	}
	scene.SetLocale(chosen)
	if !scene.HasText("Appearance") || !scene.HasText("Language") {
		t.Fatalf("settings did not stay open during the locale change: %v", scene.Texts())
	}
	clickLanguageButton(t, scene, "Language")
	scene.Key(0, ui.KeyHome)
	scene.Key(0, ui.KeyEnter)
	if chosen != "" {
		t.Fatalf("following the system left explicit preference %q", chosen)
	}
}

func clickLanguageButton(t *testing.T, scene *ui.Tester, name string) {
	t.Helper()
	button, ok := scene.AccessByRole(ui.RoleMenuButton, name)
	if !ok {
		t.Fatalf("language dropdown %q is missing: %v", name, scene.Texts())
	}
	b := button.Bounds
	scene.ClickAt(b.X+b.W/2, b.Y+b.H/2)
}

func TestLanguageDropdownShowsTheMatchedNativePreference(t *testing.T) {
	for _, tc := range []struct {
		preference, effective, settings, label, selected, system string
	}{
		{"zh-Hans", "zh-CN", "设置", "界面语言", "简体中文", "跟随系统"},
		{"en-GB", "en", "Settings", "Language", "English", "System"},
		{"fr-CA", "en", "Settings", "Language", "English", "System"},
		{"", "zh-CN", "设置", "界面语言", "跟随系统", "跟随系统"},
	} {
		t.Run(tc.preference, func(t *testing.T) {
			view := presentation.New(nil, presentation.Actions{
				ChosenLocale: func() string { return tc.preference },
				SetLocale:    func(string) {},
			})
			scene := interactionScene(view, 1020, 680)
			scene.SetLocale(tc.effective)
			chooseSource(t, scene, tc.settings)
			if !scene.HasText(tc.selected) {
				t.Errorf("language button does not show %q: %v", tc.selected, scene.Texts())
			}
			clickLanguageButton(t, scene, tc.label)
			if option, ok := scene.AccessByRole(ui.RoleMenuItemCheckBox, tc.selected); !ok || !option.Checked {
				t.Errorf("resolved choice %q is not checked: %+v", tc.selected, option)
			}
			if system, ok := scene.AccessByRole(ui.RoleMenuItemCheckBox, tc.system); !ok || system.Checked != (tc.preference == "") {
				t.Errorf("system choice does not reflect explicit preference %q: %+v", tc.preference, system)
			}
		})
	}
}
