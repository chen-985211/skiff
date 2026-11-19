package presentation

import (
	"time"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

const fullProgressHold = 180 * time.Millisecond

type progressFrame struct {
	status            string
	fraction, opacity float32
}

// Only cards seen downloading can play the finish. Keep their last painted
// fraction by task ID so switching between the grid and detail preserves it.
type visibleTransfer struct {
	status  string
	from    float32
	started time.Time
	seen    bool
}

func (v *View) forgetHiddenTransfers() {
	for id, transfer := range v.visibleTransfers {
		if !transfer.seen {
			delete(v.visibleTransfers, id)
			continue
		}
		transfer.seen = false
		v.visibleTransfers[id] = transfer
	}
}

func (v *View) cardProgress(c *ui.Context, card ui.Element, t application.TaskDTO) progressFrame {
	if t.Status != "complete" {
		fraction := card.AnimateTo("fraction", float32(t.Progress), progressEase)
		if (t.Status == "active" || t.Status == "waiting") && !c.Preferences().ReduceMotion {
			from := fraction
			if t.Total <= 0 {
				from = 1 // An indeterminate bar has no measured fraction to fill from.
			}
			v.visibleTransfers[t.ID] = visibleTransfer{status: t.Status, from: from, seen: true}
		} else {
			delete(v.visibleTransfers, t.ID)
		}
		return progressFrame{status: t.Status, fraction: fraction, opacity: 1}
	}

	transfer, ok := v.visibleTransfers[t.ID]
	if !ok {
		return progressFrame{status: "complete", fraction: 1}
	}
	if transfer.started.IsZero() {
		transfer.started = c.Now()
	}
	transfer.seen = true
	v.visibleTransfers[t.ID] = transfer

	fill := progressEase.Duration
	if transfer.from >= 1 {
		fill = 0
	}
	elapsed := c.Now().Sub(transfer.started)
	if !c.Preferences().ReduceMotion {
		switch {
		case elapsed < fill:
			c.AnimationFrame()
			tween := progressEase.Ease(float32(elapsed) / float32(fill))
			return progressFrame{status: transfer.status, fraction: transfer.from + (1-transfer.from)*tween, opacity: 1}
		case elapsed < fill+fullProgressHold:
			c.After(fill + fullProgressHold - elapsed)
			return progressFrame{status: transfer.status, fraction: 1, opacity: 1}
		case elapsed < fill+fullProgressHold+colorFade.Duration:
			c.AnimationFrame()
			fade := colorFade.Ease(float32(elapsed-fill-fullProgressHold) / float32(colorFade.Duration))
			return progressFrame{status: "complete", fraction: 1, opacity: 1 - fade}
		}
	}
	delete(v.visibleTransfers, t.ID)
	if v.Filter == "active" {
		c.Invalidate() // Let the finished card leave the filtered queue.
	}
	return progressFrame{status: "complete", fraction: 1}
}
