package transfer_test

import (
	"errors"
	"testing"
)

func TestTorrentCreationLocalizesTheReasonSeedingCouldNotStart(t *testing.T) {
	for _, tc := range []struct {
		name, english, chinese string
		existing               bool
		addErr                 error
	}{
		{
			name:     "existing download",
			english:  "The torrent was saved. Seeding could not start: Pause the download before changing the file location",
			chinese:  "种子已保存，启动做种失败：请先暂停任务，再更改文件位置",
			existing: true,
		},
		{
			name:    "unknown engine failure",
			english: "The torrent was saved. Seeding could not start: The operation failed. Try again.",
			chinese: "种子已保存，启动做种失败：操作失败，请重试",
			addErr:  errors.New("untrusted raw engine detail"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &torrentCreationEngine{fakeEngine: fakeEngine{addErr: tc.addErr}}
			scene, updates := torrentCreationScene(t, engine, tc.existing)
			chooseSource(t, scene, "Seed the original files")
			submitTorrentCreation(t, scene, updates)
			if engine.writes != 1 || !scene.HasText(tc.english) || scene.HasText("untrusted raw engine detail") {
				t.Errorf("missing safe English partial success feedback: %v", scene.Texts())
			}
			scene.SetLocale("zh-CN")
			if !scene.HasText(tc.chinese) {
				t.Errorf("missing Chinese partial success feedback: %v", scene.Texts())
			}
		})
	}
}
