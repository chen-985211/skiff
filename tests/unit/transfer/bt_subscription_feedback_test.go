package transfer_test

import (
	"context"
	"errors"
	"fmt"
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
		readURL:  application.SubscriptionHTTPError{Status: 404},
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
		{"zh-CN", "更新失败", []string{"无法读取订阅（HTTP 404）。", "订阅未返回有效 Tracker", "操作失败，请重试"}},
		{"en", "Refresh failed", []string{"Could not load the subscription (HTTP 404).", "The subscription returned no valid trackers", "The operation failed. Try again."}},
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

type rejectRefreshedTrackerRepository struct{ fakeRepo }

func (r *rejectRefreshedTrackerRepository) Save(ctx context.Context, state application.State) error {
	for _, subscription := range state.BTSubscriptions {
		if !subscription.Checked.IsZero() {
			return errors.New("private persistence detail")
		}
	}
	return r.fakeRepo.Save(ctx, state)
}

func TestManualTrackerRefreshShowsFetchAndSaveFailures(t *testing.T) {
	const missing = "https://tracker.test/missing"
	const unavailable = "https://tracker.test/unavailable"
	engine := &trackerFailureEngine{failures: map[string]error{
		missing:     application.SubscriptionHTTPError{Status: 404},
		unavailable: application.SubscriptionHTTPError{Status: 503},
	}}
	repo := &rejectRefreshedTrackerRepository{fakeRepo{state: application.State{
		Preferences: application.Preferences{BT: application.BTSettings{Sources: missing + "\n" + unavailable}},
	}}}
	svc := application.New(engine, repo, t.TempDir(), nil)
	if err := svc.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 1)
	view := presentation.New(svc, presentation.Actions{Context: t.Context(), Dispatch: func(fn func()) { updates <- fn }})
	view.Snapshot = svc.Snapshot()
	scene := interactionScene(view, 800, 560)
	for _, label := range []string{"设置", "BT 网络", "Tracker", "保存并更新"} {
		chooseSource(t, scene, label)
	}
	select {
	case update := <-updates:
		update()
		scene.Frame()
	case <-time.After(time.Second):
		t.Fatal("refresh did not finish")
	}
	for _, tc := range []struct{ locale, message, button string }{
		{"zh-CN", "无法读取订阅（HTTP 404）。\n保存任务失败\n无法读取订阅（HTTP 503）。", "保存并更新"},
		{"en", "Could not load the subscription (HTTP 404).\nCould not save the download queue\nCould not load the subscription (HTTP 503).", "Save and Refresh"},
	} {
		scene.SetLocale(tc.locale)
		bounds := sourceBounds(t, scene, ui.RoleText, tc.message)
		button := sourceBounds(t, scene, ui.RoleButton, tc.button)
		if bounds.W < 300 || bounds.H < 40 || bounds.Y+bounds.H > button.Y || button.Y+button.H > 560 {
			t.Errorf("%s refresh errors do not fit above the action: message %v, action %v", tc.locale, bounds, button)
		}
	}
}

func TestRenderLongTrackerRefreshFeedbackIsKeyboardScrollable(t *testing.T) {
	engine := &trackerFailureEngine{failures: make(map[string]error)}
	var sources, reasons []string
	for status := 400; status < 416; status++ {
		source := fmt.Sprintf("https://tracker.test/%d", status)
		sources = append(sources, source)
		reasons = append(reasons, fmt.Sprintf("Could not load the subscription (HTTP %d).", status))
		engine.failures[source] = application.SubscriptionHTTPError{Status: status}
	}
	repo := &fakeRepo{state: application.State{
		Preferences: application.Preferences{BT: application.BTSettings{Sources: strings.Join(sources, "\n")}},
	}}
	svc := application.New(engine, repo, t.TempDir(), nil)
	if err := svc.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 1)
	view := presentation.New(svc, presentation.Actions{Context: t.Context(), Dispatch: func(fn func()) { updates <- fn }})
	view.Snapshot = svc.Snapshot()
	scene := interactionScene(view, 800, 560)
	scene.SetLocale("en")
	for _, label := range []string{"Settings", "BitTorrent Network", "Tracker", "Save and Refresh"} {
		chooseSource(t, scene, label)
	}
	select {
	case update := <-updates:
		update()
		scene.Frame()
	case <-time.After(time.Second):
		t.Fatal("refresh did not finish")
	}
	const label = "Refresh failed"
	scroll := sourceBounds(t, scene, ui.RoleScroll, label)
	message := strings.Join(reasons, "\n")
	before := sourceBounds(t, scene, ui.RoleText, message)
	button := sourceBounds(t, scene, ui.RoleButton, "Save and Refresh")
	if scroll.H >= before.H || scroll.Y+scroll.H > button.Y || button.Y+button.H > 560 {
		t.Fatalf("long refresh feedback overflowed the panel: text %v, viewport %v, button %v", before, scroll, button)
	}
	for range 12 {
		if node, _ := scene.AccessByRole(ui.RoleScroll, label); node.Focused {
			break
		}
		scene.Key(0, ui.KeyTab)
	}
	scene.Key(0, ui.KeyEnd)
	after := sourceBounds(t, scene, ui.RoleText, message)
	if after.Y >= before.Y || after.Y+after.H > scroll.Y+scroll.H+1 {
		t.Errorf("keyboard scrolling did not reveal the end of the feedback: before %v, after %v, viewport %v", before, after, scroll)
	}
}
