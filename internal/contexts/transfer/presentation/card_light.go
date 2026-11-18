package presentation

import (
	"hash/crc32"
	"math"

	"github.com/egoist/mygo/ui"
)

// A card's identity sets the starting point of its own light field.
// The window clock survives reordering and the grid-to-detail transition.
func (v *View) cardSurface(c *ui.Context, card ui.Element, p palette, id string, ink ui.Color, radius float32) ui.Element {
	card.Radius(radius).Background(p.panel)
	if c.Preferences().HighContrast || c.Preferences().ReduceTransparency {
		return card.Border(1, p.border)
	}
	offset := float32(crc32.ChecksumIEEE([]byte(id))) * (2 * math.Pi / (1 << 32))
	texture := ui.Local(card, "card-light", func() lightTexture { return lightTexture{} })
	moving := !c.Preferences().ReduceMotion && !v.Snapshot.Preferences.StaticBackground
	return card.Draw(func(painter *ui.Painter, r ui.Rect) {
		// The virtual grid caches each visible row independently of its parent.
		if moving {
			requestLightFrame(painter)
		}
		painter.Clip(r, radius, func() {
			texture.draw(painter, r, cardLight, float32(v.light.phase())+offset, p.shade)
		})
		painter.Fill(r, ink.Alpha(.055), radius)
	})
}
