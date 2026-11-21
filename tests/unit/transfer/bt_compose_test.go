package transfer_test

import (
	"context"
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
	"testing"
	"time"
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
	for _, name := range []string{"监听端口", "每种节点上限"} {
		sourceBounds(t, tt, ui.RoleTextField, name)
	}
	chooseSource(t, tt, "Tracker")
	sourceBounds(t, tt, ui.RoleTextField, "Tracker 订阅源")
	chooseSource(t, tt, "做种")
	chooseSource(t, tt, "下载完成后继续做种")
	for _, name := range []string{"做种时长", "目标分享率"} {
		sourceBounds(t, tt, ui.RoleTextField, name)
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

type previewTorrentAPI struct{ *application.Service }

func (previewTorrentAPI) PrepareTorrent(context.Context, string) (application.Torrent, error) {
	return application.Torrent{Name: "样本", Hash: "fixture", Files: []application.TorrentFile{{Path: "a.txt", Size: 1}, {Path: "b.txt", Size: 1}}}, nil
}
func TestBTReplacePreviewReleasesOldFileTree(t *testing.T) {
	api := previewTorrentAPI{service(t, &fakeEngine{}, &fakeRepo{})}
	updates := make(chan func(), 1)
	v := presentation.New(api, presentation.Actions{Dispatch: func(fn func()) { updates <- fn }, PickTorrent: func(done func(string, error)) { done("/sample.torrent", nil) }})
	v.OpenNew()
	tt := interactionScene(v, 900, 600)
	for _, label := range []string{"BT", "打开种子"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case show := <-updates:
		show()
	case <-time.After(2 * time.Second):
		t.Fatal("no metadata")
	}
	tt.Frame()
	if err := tt.Click("更换种子"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTextField, "磁力链接"); !ok {
		t.Fatal("replace did not return to source")
	}
}
