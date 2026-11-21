package transfer_test

import (
	"strings"

	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/presentation"
)

func TestComposeOptionsStayInsideOneSheet(t *testing.T) {
	for _, size := range [][2]int{{800, 560}, {1020, 680}} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		v.URLs = strings.Repeat("https://example.org/archive.zip\n", 9)
		v.OpenNew()
		tt := interactionScene(v, size[0], size[1])
		click := func(label string) {
			t.Helper()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
		}
		click("高级选项")
		footer, _ := tt.Find("开始下载")
		check := func() {
			t.Helper()
			dialogs := 0
			for _, node := range tt.AccessAll() {
				if node.Role == ui.RoleDialog {
					dialogs++
					if node.Bounds.Y < 20 || node.Bounds.Y+node.Bounds.H > float32(size[1]) {
						t.Fatal("sheet exceeds window", node.Bounds)
					}
				}
			}
			if dialogs != 1 {
				t.Fatal("stacked option dialogs", dialogs)
			}
			if cur, ok := tt.Find("开始下载"); !ok || cur != footer {
				t.Fatalf("submit moved: %v -> %v", footer, cur)
			}
		}
		click("请求头")
		check()
		if h, ok := tt.AccessByRole(ui.RoleTextField, "Cookie"); !ok || !h.Password {
			t.Fatal("missing masked editable cookie")
		}
		click("文件与连接")
		field, ok := tt.AccessByRole(ui.RoleTextField, "备用下载地址")
		if !ok {
			t.Fatal("mirror editor missing")
		}
		tt.ClickAt(field.Bounds.X+15, field.Bounds.Y+12)
		tt.Type("https://mirror.test/a.zip")
		click("代理")
		check()
		click("手动代理")
		click("HTTP 代理地址")
		tt.Type("http://localhost:8080")
		if p, ok := tt.AccessByRole(ui.RoleTextField, "代理密码"); !ok || !p.Password {
			t.Fatal("manual proxy fields missing")
		}
		click("直连")
		check()
		if _, ok := tt.AccessByRole(ui.RoleTextField, "代理密码"); ok {
			t.Fatal("hidden proxy fields still accessible")
		}
		click("文件与连接")
		if m, ok := tt.AccessByRole(ui.RoleTextField, "备用下载地址"); !ok || m.Value != "https://mirror.test/a.zip" {
			t.Fatalf("switching discarded mirror draft: present=%v field=%+v", ok, m)
		}
		click("高级选项")
		if _, ok := tt.AccessByRole(ui.RoleTab, "代理"); ok {
			t.Fatal("collapsed controls still accessible")
		}
	}
}

func TestRenderComposeOptionsSelectionMovesWithoutMovingTabs(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := ui.NewTesterWith(v.Render, 800, 560, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	if err := tt.Click("高级选项"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	initial, ok := tt.AccessByRole(ui.RoleTab, "文件与连接")
	if !ok {
		t.Fatal("missing file tab")
	}
	if err := tt.Click("代理"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(75 * time.Millisecond)
	tt.Frame()
	if current, ok := tt.AccessByRole(ui.RoleTab, "文件与连接"); !ok || current.Bounds != initial.Bounds {
		t.Fatal("tab strip moved during switch")
	}
	// The active underline crosses the second tab before settling at the third.
	y := int(initial.Bounds.Y+initial.Bounds.H) - 2
	marker := func() int {
		im := tt.Image()
		best, xBest := int(1<<20), 0
		for x := int(initial.Bounds.X); x < int(initial.Bounds.X+3*initial.Bounds.W); x++ {
			pixel := im.RGBAAt(x, y)
			score := int(pixel.R) + int(pixel.G) + int(pixel.B)
			if score < best {
				best, xBest = score, x
			}
		}
		return xBest
	}
	mid := marker()
	if err := tt.Click("文件与连接"); err != nil {
		t.Fatal(err)
	}
	if at := marker(); at < mid-1 || at > mid+1 {
		t.Fatalf("reversing the selection jumped: %d -> %d", mid, at)
	}
	tt.Advance(35 * time.Millisecond)
	if err := tt.Click("代理"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	end := marker()
	if float32(mid) <= initial.Bounds.X+4 || mid >= end || float32(end) < initial.Bounds.X+initial.Bounds.W*2 {
		t.Fatalf("indicator did not travel: start=%v mid=%d end=%d", initial.Bounds, mid, end)
	}
}

func TestAddedHeaderComesIntoViewReadyToType(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	for _, label := range []string{"高级选项", "请求头", "添加自定义请求头"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	tt.Type("X-Skiff-Test")
	field, ok := tt.AccessByRole(ui.RoleTextField, "请求头名称")
	action, _ := tt.Find("开始下载")
	if !ok || !field.Focused || field.Value != "X-Skiff-Test" || field.Bounds.Y+field.Bounds.H > action.Y {
		t.Fatalf("new header not ready to edit: %+v", field)
	}
}
func TestRenderSelectedProxyModeKeepsItsLabel(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	for _, label := range []string{"高级选项", "代理", "手动代理"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	n, ok := tt.AccessByRole(ui.RoleRadio, "手动代理")
	if !ok || !n.Checked {
		t.Fatal("manual mode did not select")
	}
	im := tt.Image()
	ink := 0
	for y := int(n.Bounds.Y) + 7; y < int(n.Bounds.Y+n.Bounds.H)-7; y++ {
		for x := int(n.Bounds.X) + 10; x < int(n.Bounds.X+n.Bounds.W)-10; x++ {
			if im.RGBAAt(x, y).R < 125 {
				ink++
			}
		}
	}
	if ink < 16 {
		t.Fatal("selection highlight covered the active label", ink)
	}
}

func TestDownloadPreferencesAlignBelowConcurrency(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	tt := interactionScene(v, 800, 560)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	dir, ok := tt.AccessByRole(ui.RoleButton, "更改默认保存目录")
	if !ok {
		t.Fatal("missing directory")
	}
	queue, _ := tt.Find("同时下载数")
	for _, label := range []string{"代理与 DNS", "性能与磁盘"} {
		item, ok := tt.AccessByRole(ui.RoleButton, label)
		if !ok || item.Bounds.X != dir.Bounds.X || item.Bounds.W != dir.Bounds.W || item.Bounds.Y <= queue.Y {
			t.Fatalf("%s outside settings list: %v vs %v", label, item.Bounds, dir.Bounds)
		}
	}
}

func TestNewShortcutDoesNotStackOnNetworkSettings(t *testing.T) {
	s := service(t, &fakeEngine{}, &fakeRepo{})
	v := presentation.New(s, presentation.Actions{})
	v.Snapshot = s.Snapshot()
	tt := interactionScene(v, 800, 560)
	for _, label := range []string{"设置", "代理与 DNS"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	v.OpenNew() // The native File menu uses this same entry point.
	tt.Frame()
	if v.DialogOpen {
		t.Fatal("new download opened behind the network editor")
	}
}

func TestEscapeFromAdvancedKeepsTheDraftAndRestoresFocus(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.URLs = "https://example.test/file"
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	for _, label := range []string{"高级选项", "代理", "手动代理", "HTTP 代理地址"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !v.DialogOpen || v.URLs != "https://example.test/file" || !tt.Focused("高级选项") {
		t.Fatal("Escape lost compose context or focus")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if v.DialogOpen {
		t.Fatal("second Escape did not dismiss the sheet")
	}
}
