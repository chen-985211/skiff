package presentation

import (
	"context"
	"strconv"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/presentation/i18n"
)

func rateLabel(c *ui.Context, rate int64) string {
	switch {
	case rate <= 0:
		return tr(c, "limit.unlimited")
	case rate%(1<<20) == 0:
		return strconv.FormatInt(rate>>20, 10) + " MiB/s"
	case rate%1024 == 0:
		return strconv.FormatInt(rate>>10, 10) + " KiB/s"
	default:
		return strconv.FormatInt(rate, 10) + " B/s"
	}
}

func rateScope(id string) string {
	if id != "" {
		return "limit.task"
	}
	return "settings.globalLimit"
}

func (v *View) speedSelector(c *ui.Context, p palette, id string, current int64) {
	if save := v.rateSaves[id]; save != nil {
		current = save.current(current)
	}
	key := rateScope(id)
	label, display := tr(c, key), rateLabel(c, current)
	name := trf(c, "limit.summary", i18n.Args{"scope": label, "value": display})
	caption, tip := display, display
	width := float32(112)
	if id != "" {
		caption = trf(c, "limit.taskCaption", i18n.Args{"value": display})
		tip = name
		width = 176
	}
	dropdown(c.Key(key), p, name, caption, !v.Ready || v.Busy, func(m *ui.Menu) {
		for _, rate := range []int64{0, 512 << 10, 1 << 20, 2 << 20, 5 << 20, 10 << 20} {
			if m.Item(rateLabel(c, rate)).Checked(rate == current).Chosen() && rate != current {
				v.saveRate(id, rate)
			}
		}
		m.Separator()
		if m.Item(tr(c, "limit.custom")).Chosen() {
			v.rateID = id
			v.rateText = strconv.FormatInt(current>>10, 10)
			v.rateErr = nil
			v.rateOpen = true
		}
	}).Width(width).Tooltip(tip)
}

// Resolve the field by task ID at completion: a refresh can replace or reorder
// the task slice while the engine is applying a rate.
func (v *View) rateField(id string) *int64 {
	if id == "" {
		return &v.Snapshot.Preferences.DownloadLimit
	}
	for i := range v.Snapshot.Tasks {
		if v.Snapshot.Tasks[i].ID == id {
			return &v.Snapshot.Tasks[i].DownloadLimit
		}
	}
	return nil
}

func (v *View) saveRate(id string, rate int64) {
	if v.API == nil {
		return
	}
	save := v.rateSaves[id]
	if save == nil {
		save = &preferenceSave[int64]{}
		v.rateSaves[id] = save
	}
	if id == "" {
		v.globalRateErr = nil
	} else {
		delete(v.taskErrors, id)
	}
	save.store(v, rate, func(ctx context.Context, value int64) error {
		return v.API.SetDownloadLimit(ctx, id, value)
	}, func(value, latest int64, err error) {
		delete(v.rateSaves, id)
		field := v.rateField(id)
		if field == nil {
			return
		}
		if err == nil {
			*field = value
		}
		if latest != value && latest != *field {
			v.saveRate(id, latest)
		} else if id == "" {
			v.globalRateErr = save.err
		} else if save.err != nil {
			v.setTaskError(id, save.err)
		}
	})
}
func (v *View) rateDialog(c *ui.Context, p palette, stage ui.Element) {
	preferenceSheet(c, p, stage, "rate", tr(c, rateScope(v.rateID)), "clock", &v.rateOpen, func() {
		if preferenceNumber(c, p, tr(c, "limit.download"), "KiB/s", "0", tr(c, "limit.zeroHint"), &v.rateText).Changed() {
			v.rateErr = nil
		}
		inlineError(c, v.rateErr)
		if primaryAction(c, tr(c, "common.apply"), "check", v.Busy).AlignSelf(ui.End).Clicked() {
			n, err := strconv.ParseUint(v.rateText, 10, 53)
			if err != nil {
				v.rateErr = copyError("limit.invalid", nil)
				return
			}
			v.saveRate(v.rateID, int64(n)<<10)
			v.rateOpen = false
		}
	})
}
