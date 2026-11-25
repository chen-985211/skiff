package transfer_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

type trackerFailureEngine struct {
	fakeEngine
	failures map[string]error
}

func (e *trackerFailureEngine) FetchTrackers(_ context.Context, url string) ([]string, error) {
	return nil, e.failures[url]
}
func (*trackerFailureEngine) ChangePeerLimit(context.Context, string, int) error { return nil }
func (*trackerFailureEngine) ReadTorrentDetails(context.Context, string, string) (application.BTDetails, error) {
	return application.BTDetails{}, nil
}

func TestTrackerSubscriptionRowsExplainTheirOwnFailure(t *testing.T) {
	const readURL = "https://tracker.test/read"
	const emptyURL = "https://tracker.test/empty"
	const legacyURL = "https://tracker.test/legacy"
	const raw = "private transport detail"
	engine := &trackerFailureEngine{failures: map[string]error{
		readURL:  application.Wrap(application.ErrReadSubscription, errors.New(raw)),
		emptyURL: application.ErrEmptySubscription,
	}}
	repo := &fakeRepo{state: application.State{
		Preferences: application.Preferences{BT: application.BTSettings{Sources: strings.Join([]string{readURL, emptyURL, legacyURL}, "\n")}},
		BTSubscriptions: []application.BTSubscription{{
			URL: legacyURL, Checked: time.Now(), Error: raw,
		}},
	}}
	open := func() *application.Service {
		t.Helper()
		svc := application.New(engine, repo, t.TempDir(), nil)
		if err := svc.Open(t.Context()); err != nil {
			t.Fatal(err)
		}
		return svc
	}
	svc := open()
	if err := svc.RefreshBTSubscriptions(t.Context(), false); !errors.Is(err, application.ErrReadSubscription) || !errors.Is(err, application.ErrEmptySubscription) {
		t.Fatalf("refresh did not retain both causes: %v", err)
	}
	for _, sub := range repo.state.BTSubscriptions {
		if sub.Error != "" || sub.Failure == "" {
			t.Fatalf("expected a persisted failure code without legacy text: %+v", sub)
		}
	}
	svc = open()
	v := presentation.New(svc, presentation.Actions{})
	v.Snapshot = svc.Snapshot()
	scene := interactionScene(v, 1020, 680)
	for _, label := range []string{"设置", "BT 网络", "Tracker"} {
		chooseSource(t, scene, label)
	}
	for _, tc := range []struct {
		locale, label string
		causes        []string
	}{
		{"zh-CN", "更新失败", []string{"无法读取完整订阅", "订阅未返回有效 Tracker", "操作失败，请重试"}},
		{"en", "Refresh failed", []string{"Could not load the complete tracker subscription", "The subscription returned no valid trackers", "The operation failed. Try again."}},
	} {
		scene.SetLocale(tc.locale)
		for i, url := range []string{readURL, emptyURL, legacyURL} {
			row := sourceBounds(t, scene, ui.RoleText, url)
			var description string
			for _, node := range scene.AccessAll() {
				if node.Name == tc.label && node.Bounds.Y >= row.Y-2 && node.Bounds.Y < row.Y+row.H {
					description = node.Description
					break
				}
			}
			if description != tc.causes[i] {
				t.Errorf("%s %s description = %q, want %q", tc.locale, url, description, tc.causes[i])
			}
		}
		if scene.HasText(raw) {
			t.Error("raw diagnostic text was exposed")
		}
	}
}
