package transfer_test

import (
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

// Sample the uninterrupted area between the filename and the progress
// track. Both compact and expanded cards should have their own moving face.
func TestCardMaterialIlluminatesInterior(t *testing.T) {
	for _, tc := range []struct {
		name, status        string
		detail, dark, still bool
		prefs               ui.Preferences
	}{
		{name: "downloading", status: "active"}, {name: "paused", status: "paused"},
		{name: "complete in dark", status: "complete", dark: true},
		{name: "failed detail", status: "error", detail: true},
		{name: "static", status: "paused", still: true},
		{name: "reduce motion", status: "complete", prefs: ui.Preferences{ReduceMotion: true}},
		{name: "opaque", status: "error", prefs: ui.Preferences{ReduceTransparency: true}},
		{name: "high contrast", status: "error", prefs: ui.Preferences{HighContrast: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := presentation.New(nil, presentation.Actions{})
			v.Ready = true
			v.Snapshot.Preferences.StaticBackground = tc.still
			v.Snapshot.Tasks = []application.TaskDTO{{ID: "a", Name: "one.zip", Status: tc.status, Total: 100, Completed: 50, Progress: .5}}
			tt := ui.NewTesterWith(v.Render, 820, 540, ui.TesterOptions{Dark: tc.dark, Preferences: tc.prefs})
			if tc.detail {
				if err := tt.Click("查看详情 one.zip"); err != nil {
					t.Fatal(err)
				}
			}
			tt.Advance(700 * time.Millisecond)
			bar, ok := tt.Find("下载进度")
			if !ok {
				t.Fatal("missing progress track")
			}
			sample := func() []color.RGBA {
				im := tt.Image()
				var pixels []color.RGBA
				for y := int(bar.Y) - 28; y < int(bar.Y)-7; y += 5 {
					for x := int(bar.X + bar.W*.25); x < int(bar.X+bar.W*.89); x += 7 {
						pixels = append(pixels, im.RGBAAt(x, y))
					}
				}
				return pixels
			}
			first := sample()
			tt.Advance(1800 * time.Millisecond)
			next := sample()
			wantMotion := !tc.still && !tc.prefs.ReduceMotion && !tc.prefs.ReduceTransparency && !tc.prefs.HighContrast
			moved := changedFraction(first, next, 7)
			if wantMotion && moved < .27 || !wantMotion && moved > 0 {
				t.Errorf("moving interior %.0f%%, want motion %t", 100*moved, wantMotion)
			}
			// Put another task at the exact same place and time. A card that
			// only samples the window would give it the same background.
			snapshot := func(id string) []color.RGBA {
				v.Snapshot.Tasks[0].ID = id
				v.Snapshot.Preferences.StaticBackground = true
				fresh := presentation.New(nil, presentation.Actions{})
				fresh.Ready = true
				fresh.Snapshot = v.Snapshot
				tt = ui.NewTesterWith(fresh.Render, 820, 540, ui.TesterOptions{Dark: tc.dark, Preferences: tc.prefs})
				return sample()
			}
			if tc.name == "paused" || tc.name == "complete in dark" {
				unique := changedFraction(snapshot("a"), snapshot("another-task"), 9)
				if unique < .30 {
					t.Errorf("only %.0f%% of the interior belongs to the card", unique*100)
				}
			}
		})
	}
}

func changedFraction(before, after []color.RGBA, threshold int) float64 {
	var changed int
	for i, a := range before {
		b := after[i]
		lo := min(int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B))
		hi := max(int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B))
		if max(-lo, hi) >= threshold {
			changed++
		}
	}
	return float64(changed) / float64(len(before))
}

// In the open area below the cards, most of the contrast comes from broad
// folds. Fine reflections can come and go as the surface turns.
func TestWindowLightKeepsBroadTonalFolds(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := presentation.New(nil, presentation.Actions{})
		v.Ready = true
		tt := ui.NewTesterWith(v.Render, 900, 600, ui.TesterOptions{Dark: dark})
		tt.Advance(time.Second)
		im := tt.Image()
		var sum, squares, withinPatches, samples float64
		for left := 0; left < 900; left += 90 {
			var patch, patchSquares, size float64
			for y := 380; y < 465; y += 5 {
				for x := left + 2; x < left+90; x += 5 {
					c := im.RGBAAt(x, y)
					luma := (54*float64(c.R) + 183*float64(c.G) + 19*float64(c.B)) / 256
					patch += luma
					patchSquares += luma * luma
					size++
				}
			}
			withinPatches += patchSquares - patch*patch/size
			sum, squares, samples = sum+patch, squares+patchSquares, samples+size
		}
		variation := squares - sum*sum/samples
		if variation < samples*8*8 {
			t.Errorf("dark=%t: ambient light field has too little contrast", dark)
		} else if withinPatches > .30*variation {
			t.Errorf("dark=%t: broad folds account for only %.0f%% of the contrast", dark, 100*(1-withinPatches/variation))
		}
	}
}

func TestCardMaterialExpandsAndReversesFromCurrentSize(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.Snapshot.Tasks = []application.TaskDTO{
		{ID: "a", Name: "one.zip", Status: "paused", CanResume: true},
		{ID: "b", Name: "two.zip", Status: "active", CanPause: true},
	}
	tt := ui.NewTesterWith(v.Render, 1020, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	// Only the paused task has this action, even when it is the second
	// card of a row. Track the same piece of content in both layouts.
	bounds := func() ui.Rect {
		t.Helper()
		r, ok := tt.Find("继续下载")
		if !ok {
			t.Fatal("missing paused task's action")
		}
		return r
	}
	open := func() {
		t.Helper()
		if err := tt.Click("查看详情 one.zip"); err != nil {
			t.Fatal(err)
		}
	}
	grid := bounds()
	open()
	start := bounds()
	tt.Advance(95 * time.Millisecond)
	middle := bounds()
	tt.Advance(time.Second)
	detail := bounds()
	if math.Abs(float64(start.W-grid.W)) > .1*float64(detail.W-grid.W) ||
		!(start.W+5 < middle.W && middle.W+5 < detail.W && start.X > middle.X && middle.X > detail.X) {
		t.Fatalf("expansion: grid %v, start %v, middle %v, detail %v", grid, start, middle, detail)
	}
	if err := tt.Click("收起详情 one.zip"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	open()
	tt.Advance(95 * time.Millisecond)
	before := bounds()
	if err := tt.Click("收起详情 one.zip"); err != nil {
		t.Fatal(err)
	}
	after := bounds()
	if math.Abs(float64(after.W-before.W)) > .1*float64(detail.W-grid.W) ||
		math.Abs(float64(after.X-before.X)) > .1*float64(grid.X-detail.X) {
		t.Fatalf("interrupted expansion jumped: %v -> %v", before, after)
	}
	tt.Advance(time.Second)
	if end := bounds(); end != grid {
		t.Fatalf("returned to %v, want original %v", end, grid)
	}
}
