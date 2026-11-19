package transfer_test

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestWorkspaceDetailUsesAFourCellFootprint(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		width, height, count, index int
		left, up                    bool
		wantW                       float32
	}{
		{"middle", 1440, 920, 12, 4, false, false, 684},
		{"right edge", 1440, 920, 12, 5, true, false, 684},
		{"bottom middle", 1440, 920, 12, 10, false, true, 684},
		{"bottom right", 1440, 920, 12, 11, true, true, 684},
		{"partial bottom row", 1440, 920, 8, 7, false, true, 684},
		{"single row", 2016, 680, 2, 1, false, false, 684},
		{"viewport bottom", 1440, 680, 18, 8, true, true, 684},
		{"one column", 800, 680, 8, 0, false, false, 574},
	} {
		v := presentation.New(nil, presentation.Actions{})
		for i := tc.count - 1; i >= 0; i-- {
			name := fmt.Sprintf("task-%d.zip", i)
			v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: name, Name: name, Status: "complete"})
		}
		tt := interactionScene(v, tc.width, tc.height)
		name := fmt.Sprintf("task-%d.zip", tc.index)
		peer, _ := tt.Find("查看详情 task-1.zip")
		source, _ := tt.AccessByRole(ui.RoleGroup, name)
		if err := tt.Click("查看详情 " + name); err != nil {
			t.Fatal(err)
		}
		detail, ok := tt.AccessByRole(ui.RoleGroup, name)
		want := ui.Rect{X: source.Bounds.X, Y: source.Bounds.Y, W: tc.wantW, H: 324}
		if tc.left {
			want.X += source.Bounds.W - want.W
		}
		if tc.up {
			want.Y += source.Bounds.H - want.H
		}
		if !ok || detail.Bounds != want {
			t.Errorf("%s: from %v, got %v, want %v", tc.name, source.Bounds, detail.Bounds, want)
		}
		if _, shown := tt.Find("查看详情 task-1.zip"); shown {
			t.Error("another preview is still visible in the detail view")
		}
		if err := tt.Click("返回任务列表"); err != nil {
			t.Fatal(err)
		}
		end, _ := tt.AccessByRole(ui.RoleGroup, name)
		if restored, ok := tt.Find("查看详情 task-1.zip"); !ok || restored != peer {
			t.Errorf("neighbor returned to %v, want %v", restored, peer)
		}
		if end.Bounds != source.Bounds {
			t.Errorf("returned to %v, want %v", end.Bounds, source.Bounds)
		}
	}
}

func TestWorkspaceDetailAnchorsToScrolledSearchResults(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Query, v.EngineError = "match", errors.New("无法连接下载引擎")
	for i := 15; i >= 0; i-- {
		name := fmt.Sprintf("match-%d.zip", i)
		v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: name, Name: name, Status: "complete"})
	}
	v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: "other", Name: "other.zip"})
	tt := interactionScene(v, 1440, 760)
	tt.Scroll(600, 400, 0, 175)
	const name = "match-11.zip"
	source, found := tt.AccessByRole(ui.RoleGroup, name)
	if !found {
		t.Fatal("missing scrolled card")
	}
	// The file action opens the same detail as a click on the card body.
	tt.ClickAt(source.Bounds.X+45, source.Bounds.Y+source.Bounds.H-25)
	detail, found := tt.AccessByRole(ui.RoleGroup, name)
	if !tt.HasText("来源") || !found || detail.Bounds != (ui.Rect{X: source.Bounds.X - 348, Y: source.Bounds.Y - 168, W: 684, H: 324}) {
		t.Fatalf("scrolled bottom-right card %v opened at %v", source.Bounds, detail.Bounds)
	}
	tt.Key(0, ui.KeyEscape)
	if end, ok := tt.AccessByRole(ui.RoleGroup, name); !ok || end.Bounds != source.Bounds {
		t.Fatalf("scroll position changed on return: %v -> %v", source.Bounds, end.Bounds)
	}
}

func TestCardMaterialReopensDuringContraction(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	for i := range 9 {
		name := fmt.Sprintf("task-%d.zip", i)
		v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: name, Name: name, Status: "complete"})
	}
	tt := ui.NewTesterWith(v.Render, 1440, 800, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Advance(time.Second)
	bounds := func() ui.Rect {
		t.Helper()
		card, found := tt.AccessByRole(ui.RoleGroup, "task-0.zip")
		if !found {
			t.Fatal("selected card disappeared")
		}
		return card.Bounds
	}
	source := bounds()
	if err := tt.Click("查看详情 task-0.zip"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(145 * time.Millisecond)
	tt.Frame()
	out := bounds()
	if out.W <= source.W || out.H <= source.H || out.W >= 684 || out.H >= 324 {
		t.Fatalf("no intermediate expansion: %v -> %v", source, out)
	}
	tt.Key(0, ui.KeyEscape)
	if r := bounds(); math.Abs(float64(r.H-out.H)) > 1 {
		t.Fatalf("interruption jumped: %v -> %v", out, r)
	}
	tt.Advance(95 * time.Millisecond)
	tt.Frame()
	in := bounds()
	if in.W >= out.W || in.H >= out.H || in.W <= source.W {
		t.Fatalf("did not contract from %v: %v", out, in)
	}
	for _, frame := range []ui.Rect{out, in} {
		if math.Abs(float64(frame.X+frame.W-source.X-source.W)) > 1 || math.Abs(float64(frame.Y+frame.H-source.Y-source.H)) > 1 {
			t.Errorf("bottom-right corner drifted between %v and %v", source, frame)
		}
	}
	tt.ClickAt(in.X+in.W/2, in.Y+in.H/2)
	if r := bounds(); math.Abs(float64(r.W-in.W)) > 1 {
		t.Fatalf("reopen jumped: %v -> %v", in, r)
	}
	tt.Advance(time.Second)
	if r := bounds(); r.W != 684 || r.H != 324 || r.X+r.W != source.X+source.W || r.Y+r.H != source.Y+source.H {
		t.Fatalf("did not resume expansion: %v", r)
	}
}
