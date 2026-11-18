package transfer_test

import (
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
		width, index int
		wantW        float32
	}{
		{2016, 0, 684}, {2016, 4, 684}, {1020, 3, 684}, {800, 0, 574},
	} {
		v := presentation.New(nil, presentation.Actions{})
		for i := 7; i >= 0; i-- {
			name := fmt.Sprintf("task-%d.zip", i)
			v.Snapshot.Tasks = append(v.Snapshot.Tasks, application.TaskDTO{ID: name, Name: name, Status: "complete"})
		}
		tt := ui.NewTesterWith(v.Render, tc.width, 680, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
		name := fmt.Sprintf("task-%d.zip", tc.index)
		first, _ := tt.AccessByRole(ui.RoleGroup, "task-0.zip")
		peer, _ := tt.Find("查看详情 task-1.zip")
		source, _ := tt.AccessByRole(ui.RoleGroup, name)
		if err := tt.Click("查看详情 " + name); err != nil {
			t.Fatal(err)
		}
		detail, ok := tt.AccessByRole(ui.RoleGroup, name)
		if !ok || detail.Bounds != (ui.Rect{X: first.Bounds.X, Y: first.Bounds.Y, W: tc.wantW, H: 324}) {
			t.Errorf("%d pt window, %s from %v: unexpected detail bounds %v", tc.width, name, source.Bounds, detail.Bounds)
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

func TestCardMaterialReopensDuringContraction(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one.zip", Status: "complete"}, {ID: "b", Name: "two.zip", Status: "complete"}}
	tt := ui.NewTester(v.Render, 1020, 680)
	tt.Advance(time.Second)
	bounds := func() ui.Rect {
		t.Helper()
		card, found := tt.AccessByRole(ui.RoleGroup, "one.zip")
		if !found {
			t.Fatal("selected card disappeared")
		}
		return card.Bounds
	}
	source := bounds()
	if err := tt.Click("查看详情 one.zip"); err != nil {
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
	tt.ClickAt(in.X+in.W/2, in.Y+in.H/2)
	if r := bounds(); math.Abs(float64(r.W-in.W)) > 1 {
		t.Fatalf("reopen jumped: %v -> %v", in, r)
	}
	tt.Advance(time.Second)
	if r := bounds(); r.W != 684 || r.H != 324 {
		t.Fatalf("did not resume expansion: %v", r)
	}
}
