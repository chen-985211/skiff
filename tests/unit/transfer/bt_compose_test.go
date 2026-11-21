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
func TestBTSettingsOpensFromSettings(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	v := presentation.New(s, presentation.Actions{})
	tt := interactionScene(v, 1020, 680)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("BT 网络"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Tracker 订阅源", "监听端口", "每种节点上限", "做种时长", "目标分享率"} {
		if _, ok := tt.AccessByRole(ui.RoleTextField, name); !ok {
			t.Fatal("missing", name)
		}
	}
}

func TestBTCreateEditorKeepsDownloadDraft(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	v := presentation.New(s, presentation.Actions{})
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	click := func(name string) {
		t.Helper()
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	click("BT")
	click("BT 选项")
	click("创建种子…")
	if _, ok := tt.AccessByRole(ui.RoleTextField, "制种来源"); !ok {
		t.Fatal("missing source")
	}
	click("返回下载")
	if _, ok := tt.AccessByRole(ui.RoleTextField, "磁力链接"); !ok {
		t.Fatal("lost torrent download editor")
	}
}
