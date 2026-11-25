package transfer_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

func TestSettingsPlacesAppearanceBeforeDownloadAndNetwork(t *testing.T) {
	v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{
		ChosenLocale: func() string { return "zh-CN" },
		SetLocale:    func(string) {},
	})
	tt := interactionScene(v, 800, 560)
	chooseSource(t, tt, "设置")
	previous := sourceBounds(t, tt, ui.RoleText, "设置")
	for _, item := range []struct {
		role ui.Role
		name string
	}{
		{ui.RoleText, "外观"},
		{ui.RoleRadioGroup, "主题"},
		{ui.RoleSwitch, "背景流光"},
		{ui.RoleMenuButton, "界面语言"},
		{ui.RoleText, "下载"},
		{ui.RoleText, "网络"},
	} {
		current := sourceBounds(t, tt, item.role, item.name)
		if current.Y < previous.Y+previous.H {
			t.Errorf("%s at %v should follow %v", item.name, current, previous)
		}
		previous = current
	}
}

func TestSettingsOverviewFitsDesktopAndGroupsNetworkControls(t *testing.T) {
	for _, size := range [][2]int{{800, 560}, {912, 594}, {1020, 680}} {
		v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
		tt := interactionScene(v, size[0], size[1])
		chooseSource(t, tt, "设置")
		proxy := sourceBounds(t, tt, ui.RoleButton, "代理与 DNS")
		bt := sourceBounds(t, tt, ui.RoleButton, "BT 网络")
		if !tt.HasText("网络") || bt.Y-proxy.Y-proxy.H > 2 || bt.X != proxy.X {
			t.Errorf("%v: proxy and BT should share one network section: %v, %v", size, proxy, bt)
		}
		if bt.Y+bt.H+8 > float32(size[1]) {
			t.Errorf("%v: last setting is clipped at %v", size, bt)
		}
		tt.Scroll(bt.X+20, bt.Y+8, 0, 280)
		tt.Frame()
		if after := sourceBounds(t, tt, ui.RoleButton, "BT 网络"); after.Y != bt.Y {
			t.Errorf("%v: overview overflows by %.1f", size, bt.Y-after.Y)
		}
	}
}

func TestSettingsOverflowCanBeDraggedOutsideRows(t *testing.T) {
	v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
	tt := interactionScene(v, 800, 410)
	chooseSource(t, tt, "设置")
	dir := sourceBounds(t, tt, ui.RoleButton, "更改默认保存目录")
	lastRow := func() ui.Rect { return sourceBounds(t, tt, ui.RoleButton, "BT 网络") }
	if r := lastRow(); r.Y+r.H <= 410 {
		t.Fatalf("last setting should start outside the visible area: %v", r)
	}
	// Use the content gutter, clear of the row and its action.
	x := dir.X + dir.W + 10
	tt.Move(dir.X+20, dir.Y+10)
	tt.Advance(time.Second)
	tt.Move(x, 90)
	tt.Frame()
	tt.Press(x, 90)
	tt.Move(x, 406)
	tt.Frame()
	tt.Release(x, 406)
	tt.Frame()
	if r := lastRow(); r.Y+r.H > 410 {
		t.Errorf("dragging beyond the card failed to reach the last control: %v", r)
	}
}
