package transfer_test

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

func downloadPreference(t *testing.T, labels ...string) *ui.Tester {
	t.Helper()
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := interactionScene(v, 800, 560)
	for _, label := range append([]string{"设置"}, labels...) {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	return tt
}

func TestCustomRateHasContextAndAnAdjacentUnit(t *testing.T) {
	tt := downloadPreference(t, "全局限速 · 不限速")
	if !tt.HasText("512 KiB/s") || !tt.HasText("1 MiB/s") {
		t.Error("presets should use the same units as the custom editor")
	}
	if err := tt.Click("自定义…"); err != nil {
		t.Fatal(err)
	}
	_, titled := tt.AccessByRole(ui.RoleDialog, "全局限速")
	field, exists := tt.AccessByRole(ui.RoleTextField, "下载上限")
	unit, ok := tt.Find("KiB/s")
	if !titled || !exists || !ok || unit.X < field.Bounds.X || unit.X+unit.W > field.Bounds.X+field.Bounds.W {
		t.Fatal("the custom rate needs a named dialog with KiB/s in its numeric field")
	}
	if err := tt.Click("关闭全局限速"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.AccessByRole(ui.RoleDialog, ""); ok {
		t.Fatal("close should discard the draft")
	}
}

func TestNetworkSettingsAlignsProxyAndDNSControls(t *testing.T) {
	tt := downloadPreference(t, "代理与 DNS", "手动代理")
	panel, named := tt.AccessByRole(ui.RoleDialog, "代理与 DNS")
	address, found := tt.AccessByRole(ui.RoleTextField, "HTTP 代理地址")
	dns, ok := tt.AccessByRole(ui.RoleMenuButton, "DNS · 自动")
	rule, hasRule := tt.AccessByRole(ui.RoleSeparator, "")
	if !named || !found || !ok || !hasRule || rule.Bounds.Y >= dns.Bounds.Y || rule.Bounds.Y <= address.Bounds.Y ||
		address.Bounds.X != dns.Bounds.X || address.Bounds.W != dns.Bounds.W || panel.Bounds.W < dns.Bounds.W {
		t.Fatalf("proxy form and DNS need aligned fields and separate groups: address %v DNS %v separator %v", address.Bounds, dns.Bounds, rule.Bounds)
	}
	if err := tt.Click("关闭代理与 DNS"); err != nil {
		t.Fatal(err)
	}
}
