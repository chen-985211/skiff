package presentation

import (
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation/i18n"
	"slices"
	"strconv"
	"strings"
)

type requestField struct{ Name, Value string }

func (v *View) headerBlock() string {
	var headers []string
	for _, h := range v.fields {
		if h.Value != "" {
			headers = append(headers, h.Name+": "+h.Value)
		}
	}
	return strings.Join(headers, "\n")
}
func (v *View) loadHeaders(block string) {
	v.fields = nil
	for _, name := range []string{"Cookie", "Authorization", "Referer", "User-Agent"} {
		v.fields = append(v.fields, requestField{Name: name})
	}
	for line := range strings.Lines(block) {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		field := requestField{Name: key, Value: strings.TrimSpace(value)}
		added := false
		for i, h := range v.fields[:4] {
			if strings.EqualFold(key, h.Name) && h.Value == "" {
				v.fields[i].Value = field.Value
				added = true
				break
			}
		}
		if !added {
			v.fields = append(v.fields, field)
		}
	}
}
func (v *View) headerOptions(c *ui.Context, p palette) {
	if v.fields == nil {
		v.loadHeaders("")
	}
	added := false
	rows := ui.Column(c.Key("header-fields")).FillWidth().Gap(7)
	// Build the actions first so an added row can take focus in this frame.
	// The rows already own their place above this trailing toolbar.
	ui.Row(c).FillWidth().Height(28).Shrink(0).AlignItems(ui.Center).Justify(ui.End).Gap(8).Children(func() {
		if headerAction(c, p, "paste", tr(c, "headers.import"), tr(c, "headers.importClipboard")) {
			block, err := application.ImportHeaders(c.ReadClipboard())
			v.headerErr = err
			if err == nil && block != "" {
				v.loadHeaders(block)
			}
		}
		if headerAction(c, p, "plus", tr(c, "headers.add"), tr(c, "headers.addCustom")) {
			v.fields = append(v.fields, requestField{})
			added = true
		}
	})
	inlineError(c, v.headerErr)
	remove := -1
	rows.Children(func() {
		for i := range v.fields {
			h := &v.fields[i]
			if i < 4 {
				formRow(c, p, h.Name, func() { rowInput(c, p, h.Name, "", &h.Value, i < 2) })
				continue
			}
			ui.Row(c.Key("custom-" + strconv.Itoa(i))).Transition(pageFade).FillWidth().Height(32).Shrink(0).Gap(optionGap).AlignItems(ui.Center).Children(func() {
				name := rowInput(c, p, tr(c, "headers.nameLabel"), tr(c, "headers.name"), &h.Name, false).Grow(0).Basis(optionLabelWidth).MinWidth(optionLabelWidth)
				if added && i == len(v.fields)-1 {
					name.Focus().ScrollIntoView()
				}
				rowInput(c, p, tr(c, "headers.valueLabel"), tr(c, "headers.value"), &h.Value, true)
				if iconButton(c, "close", trf(c, "headers.removeCustom", i18n.Args{"index": i - 3}), false) {
					remove = i
				}
			})
		}
	})
	if remove >= 0 {
		v.fields = slices.Delete(v.fields, remove, remove+1)
	}
}

func headerAction(c *ui.Context, p palette, symbol, caption, label string) bool {
	b := ui.ButtonBase(c.Key(label)).Transition(controlMotion).Label(label).Tooltip(label).
		Height(28).PaddingX(8).Gap(5).Radius(cornerSmall).FocusRing(true).FocusRingOffset(-2)
	if b.Hovered() {
		b.Background(p.panel.Alpha(.55))
	}
	b.Children(func() {
		ui.Icon(c, icons[symbol]).FontSize(14).TextColor(p.active).AccessHidden(true)
		ui.Text(c, caption).SingleLine().FontSize(12).FontWeight(550).TextColor(p.active)
	})
	return b.Clicked()
}
