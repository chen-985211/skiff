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

//go:embed art/card-light.kage
var cardLightSource string

var (
	light     = shader.MustCompile(lightSource)
	cardLight = shader.MustCompile(cardLightSource)
)

type windowLight struct {
	texture   lightTexture
	lastFrame time.Time
	elapsed   time.Duration
	moving    bool
}

const lightCycle = 18 * time.Second

func (l *windowLight) phase() float64 {
	return l.elapsed.Seconds() * (2 * math.Pi) / lightCycle.Seconds()
}

func (l *windowLight) draw(p *ui.Painter, r ui.Rect, still bool, shade float32) {
	now := p.Now()
	if l.moving && !still {
		l.elapsed = (l.elapsed + now.Sub(l.lastFrame)) % lightCycle
	}
	l.lastFrame, l.moving = now, !still
	phase := float32(l.phase())
	if !still {
		requestLightFrame(p)
	}
	l.texture.draw(p, r, light, phase, shade)
}

// Native renderers shade the actual surface. A smoothly sampled canvas
// keeps the same broad folds inexpensive on a software renderer.
type lightTexture struct {
	software                 *ui.Canvas
	cachedPhase, cachedShade float32
}

func (t *lightTexture) draw(p *ui.Painter, r ui.Rect, program *shader.Shader, phase, shade float32) {
	opts := shader.Options{Uniforms: map[string]any{"Phase": phase, "Dark": shade}}
	if p.Accelerated() {
		program.Draw(p, r, opts)
		return
	}
	const w, h = 64, 44
	if t.software == nil || t.cachedPhase != phase || t.cachedShade != shade {
		if t.software == nil {
			t.software = ui.NewCanvas(w, h)
		}
		t.software.Draw(p, func(cp *ui.Painter) { program.Draw(cp, ui.Rect{W: w, H: h}, opts) })
		t.cachedPhase, t.cachedShade = phase, shade
	}
	p.Image(t.software.Bitmap(), r, ui.FillBox)
}

func requestLightFrame(p *ui.Painter) {
	if p.Accelerated() {
		p.AnimationFrameRate(30)
	} else {
		p.AnimationFrameRate(18)
	}
}
