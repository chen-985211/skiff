package presentation

import (
	_ "embed"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

//go:embed art/light.kage
var lightSource string

var light = shader.MustCompile(lightSource)

// The light has no fine texture. A small, smoothly sampled canvas is enough
// for software renderers; native windows shade the full scene on the GPU.
type windowLight struct {
	software    *ui.Canvas
	lastFrame   time.Time
	elapsed     time.Duration
	cachedPhase float32
	cachedDark  bool
	moving      bool
}

func (l *windowLight) draw(p *ui.Painter, r ui.Rect, still, dark bool) {
	now := p.Now()
	const cycle = 18 * time.Second
	if l.moving && !still {
		l.elapsed = (l.elapsed + now.Sub(l.lastFrame)) % cycle
	}
	l.lastFrame, l.moving = now, !still
	phase := float32(l.elapsed.Seconds() * (2 * math.Pi) / cycle.Seconds())
	if !still {
		if p.Accelerated() {
			p.AnimationFrameRate(30)
		} else {
			p.AnimationFrameRate(18)
		}
	}
	shade := float32(0)
	if dark {
		shade = 1
	}
	opts := shader.Options{Uniforms: map[string]any{"Phase": phase, "Dark": shade}}
	if p.Accelerated() {
		light.Draw(p, r, opts)
		return
	}
	const w, h = 64, 44
	if l.software == nil || l.cachedPhase != phase || l.cachedDark != dark {
		if l.software == nil {
			l.software = ui.NewCanvas(w, h)
		}
		l.software.Draw(p, func(cp *ui.Painter) { light.Draw(cp, ui.Rect{W: w, H: h}, opts) })
		l.cachedPhase, l.cachedDark = phase, dark
	}
	p.Image(l.software.Bitmap(), r, ui.FillBox)
}
