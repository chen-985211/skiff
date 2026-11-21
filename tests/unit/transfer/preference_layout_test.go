package transfer_test

import (
	"math"
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
	if !titled || !exists || !ok || unit.X < field.Bounds.X || unit.X+unit.W > field.Bounds.X+field.Bounds.W || math.Abs(float64(unit.Y+unit.H/2-field.Bounds.Y-field.Bounds.H/2)) > 1 {
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

func TestDiskSettingsShareAColumnAndRestoreDefaultsInPlace(t *testing.T) {
	tt := downloadPreference(t, "性能与磁盘")
	panel, titled := tt.AccessByRole(ui.RoleDialog, "性能与磁盘")
	cache, c := tt.AccessByRole(ui.RoleTextField, "磁盘缓存")
	split, s := tt.AccessByRole(ui.RoleTextField, "最小分段")
	alloc, a := tt.AccessByRole(ui.RoleMenuButton, "文件预分配 · 自动")
	reset, r := tt.AccessByRole(ui.RoleButton, "恢复默认")
	save, ok := tt.AccessByRole(ui.RoleButton, "保存")
	if !titled || !c || !s || !a || !r || !ok || cache.Bounds.X != split.Bounds.X || cache.Bounds.X != alloc.Bounds.X ||
		cache.Bounds.W != split.Bounds.W || cache.Bounds.W != alloc.Bounds.W || reset.Bounds.X > panel.Bounds.X+30 || save.Bounds.X <= reset.Bounds.X+reset.Bounds.W {
		t.Fatalf("disk inputs and actions aren't aligned: %v %v %v / %v %v", cache.Bounds, split.Bounds, alloc.Bounds, reset.Bounds, save.Bounds)
	}
	if err := tt.Click("文件预分配 · 自动"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("预分配"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.AccessByRole(ui.RoleMenuButton, "文件预分配 · 预分配"); !ok {
		t.Fatal("allocation did not update")
	}
	if err := tt.Click("恢复默认"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.AccessByRole(ui.RoleMenuButton, "文件预分配 · 自动"); !ok {
		t.Fatal("reset should restore automatic allocation in the same dialog")
	}
}
