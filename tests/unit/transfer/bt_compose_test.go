package transfer_test

import (
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
	"testing"
)

func TestBTTabKeepsSeparateLinkDraft(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	v := presentation.New(s, presentation.Actions{})
	v.URLs = "https://example.test/file"
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("BT"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "磁力链接"); !ok {
		t.Fatal("missing magnet entry")
	}
	if err := tt.Click("链接"); err != nil {
		t.Fatal(err)
	}
	if n, ok := tt.AccessByRole(ui.RoleTextField, "下载链接"); !ok || n.Value != v.URLs {
		t.Fatal("lost link draft")
	}
}
