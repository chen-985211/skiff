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
