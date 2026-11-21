package transfer_test

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestBTSettingsScrollsOnlySubscriptionResults(t *testing.T) {
	v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
	for i := range 30 {
		v.Snapshot.BTSubscriptions = append(v.Snapshot.BTSubscriptions, application.BTSubscription{URL: fmt.Sprintf("https://tracker.test/feeds/%d", i)})
	}
	tt := interactionScene(v, 800, 560)
	for _, label := range []string{"设置", "BT 网络", "Tracker"} {
		chooseSource(t, tt, label)
	}
	editor := sourceBounds(t, tt, ui.RoleTextField, "Tracker 订阅源")
	save := sourceBounds(t, tt, ui.RoleButton, "保存并更新")
	list := sourceBounds(t, tt, ui.RoleList, "订阅状态")
	first := sourceBounds(t, tt, ui.RoleText, v.Snapshot.BTSubscriptions[0].URL)
	if list.Y < editor.Y+editor.H || list.Y+list.H > save.Y || first.X > editor.X || editor.X-first.X > 1 {
		t.Fatalf("list overlaps editor/footer or loses alignment: %v %v %v %v", editor, list, first, save)
	}
	third := sourceBounds(t, tt, ui.RoleText, v.Snapshot.BTSubscriptions[2].URL)
	fourth := sourceBounds(t, tt, ui.RoleText, v.Snapshot.BTSubscriptions[3].URL)
	if third.Y+third.H > list.Y+list.H || fourth.Y < list.Y+list.H {
		t.Errorf("the initial viewport cuts through a subscription: %v, %v, viewport %v", third, fourth, list)
	}
	tt.Scroll(list.X+list.W/2, list.Y+list.H/2, 0, 2000)
	tt.Frame()
	last := sourceBounds(t, tt, ui.RoleText, v.Snapshot.BTSubscriptions[29].URL)
	if last.Y < list.Y || last.Y+last.H > list.Y+list.H+1 ||
		sourceBounds(t, tt, ui.RoleTextField, "Tracker 订阅源") != editor || sourceBounds(t, tt, ui.RoleButton, "保存并更新") != save {
		t.Fatalf("last subscription inaccessible or form moved: %v %v", list, last)
	}
}

func TestRenderBTSettingsTabsFadeAndReverse(t *testing.T) {
	v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
	tt := ui.NewTesterWith(v.Render, 912, 594, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	chooseSource(t, tt, "设置")
	chooseSource(t, tt, "BT 网络")
	tt.Advance(time.Second)
	save := sourceBounds(t, tt, ui.RoleButton, "保存并更新")
	port := sourceBounds(t, tt, ui.RoleTextField, "监听端口")
	r := image.Rect(int(port.X)+1, int(port.Y)+2, int(port.X)+65, int(port.Y+port.H)-2)
	sample := func() []byte { return renderedRegion(tt, r) }
	start := sample()
	chooseSource(t, tt, "做种")
	if !bytes.Equal(start, sample()) {
		t.Fatal("discarded outgoing page on press")
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "监听端口"); ok {
		t.Fatal("departing page is still interactive")
	}
	tt.Advance(55 * time.Millisecond)
	tt.Frame()
	mid := sample()
	chooseSource(t, tt, "连接")
	if !bytes.Equal(mid, sample()) {
		t.Fatal("reversal jumped")
	}
	tt.Advance(time.Second)
	if bytes.Equal(mid, start) || !bytes.Equal(start, sample()) {
		t.Fatal("page failed to fade and settle on return")
	}
	chooseSource(t, tt, "做种")
	tt.Advance(time.Second)
	if save != sourceBounds(t, tt, ui.RoleButton, "保存并更新") {
		t.Fatal("footer moved on tab switch")
	}
}

func TestBTSettingsKeyboardDraftsAndValidation(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	done := make(chan func(), 1)
	v := presentation.New(s, presentation.Actions{Dispatch: func(fn func()) { done <- fn }})
	tt := interactionScene(v, 800, 560)
	chooseSource(t, tt, "设置")
	chooseSource(t, tt, "BT 网络")
	edit := func(name, value string) {
		t.Helper()
		r := sourceBounds(t, tt, ui.RoleTextField, name)
		tt.ClickAt(r.X+12, r.Y+r.H/2)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(value)
	}
	edit("监听端口", "oops")
	chooseSource(t, tt, "做种")
	if _, ok := tt.AccessByRole(ui.RoleTextField, "目标分享率"); ok {
		t.Fatal("disabled seeding exposes stop conditions")
	}
	chooseSource(t, tt, "下载完成后继续做种")
	edit("做种时长", "30")
	chooseSource(t, tt, "保存并更新")
	if n, ok := tt.AccessByRole(ui.RoleTextField, "监听端口"); !ok || n.Value != "oops" {
		t.Fatal("invalid draft should be shown on its own page")
	}
	edit("监听端口", "41000")
	chooseSource(t, tt, "Tracker")
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if !tt.Focused("做种") {
		t.Fatal("arrow key did not move tab focus")
	}
	if n, ok := tt.AccessByRole(ui.RoleTextField, "做种时长"); !ok || n.Value != "30" {
		t.Fatal("switch lost draft")
	}
	chooseSource(t, tt, "保存并更新")
	select {
	case apply := <-done:
		apply()
		tt.Frame()
	case <-time.After(2 * time.Second):
		t.Fatal("save did not finish")
	}
	got := s.Snapshot().Preferences.BT
	if got.ListenPort != 41000 || !got.Seed.Enabled || got.Seed.Minutes != 30 {
		t.Fatalf("wrong saved settings: %+v", got)
	}
}

func TestRenderBTSeedLimitsEnterAndExit(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
		tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true, ReduceMotion: reduced}})
		for _, label := range []string{"设置", "BT 网络", "做种"} {
			chooseSource(t, tt, label)
		}
		tt.Advance(time.Second)
		toggle := "下载完成后继续做种"
		chooseSource(t, tt, toggle)
		tt.Advance(time.Second)
		field := sourceBounds(t, tt, ui.RoleTextField, "目标分享率")
		crop := image.Rect(int(field.X)+4, int(field.Y)+2, int(field.X)+60, int(field.Y+field.H)-2)
		sample := func() []byte { return renderedRegion(tt, crop) }
		shown := sample()
		chooseSource(t, tt, toggle)
		if _, ok := tt.AccessByRole(ui.RoleTextField, "目标分享率"); ok {
			t.Fatal("hidden conditions still take input")
		}
		first := sample()
		tt.Advance(65 * time.Millisecond)
		mid := sample()
		tt.Advance(time.Second)
		hidden := sample()
		chooseSource(t, tt, toggle)
		enter := sample()
		tt.Advance(time.Second)
		if bytes.Equal(shown, hidden) || !bytes.Equal(shown, sample()) {
			t.Fatal("limits didn't return to their previous state")
		}
		if reduced {
			if !bytes.Equal(first, hidden) || !bytes.Equal(enter, shown) {
				t.Error("reduced motion didn't settle immediately")
			}
		} else if !bytes.Equal(first, shown) || bytes.Equal(mid, shown) || bytes.Equal(mid, hidden) || !bytes.Equal(enter, hidden) {
			t.Error("seed controls jumped on entry or exit")
		}
	}
}

func TestRenderBTSubscriptionsOpenAtFirstAddress(t *testing.T) {
	visibleLines := "https://first.test/feed\nhttps://second.test/feed\nhttps://third.test/feed"
	header := func(sources string) []byte {
		v := presentation.New(service(t, &fakeEngine{}, &fakeRepo{}), presentation.Actions{})
		v.Snapshot.Preferences.BT.Sources = sources
		tt := interactionScene(v, 800, 560)
		for _, label := range []string{"设置", "BT 网络", "Tracker"} {
			chooseSource(t, tt, label)
		}
		r := sourceBounds(t, tt, ui.RoleTextField, "Tracker 订阅源")
		return renderedRegion(tt, image.Rect(int(r.X)+4, int(r.Y)+2, int(r.X)+150, int(r.Y+r.H)-1))
	}
	if !bytes.Equal(header(visibleLines), header(visibleLines+strings.Repeat("\nhttps://last.test/feed", 20))) {
		t.Error("long drafts should open at the first address and display only complete lines")
	}
}
