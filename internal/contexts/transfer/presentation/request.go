package presentation

import (
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
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
	ui.Row(c).FillWidth().Height(28).Shrink(0).AlignItems(ui.Center).Gap(8).Children(func() {
		count := 0
		for _, h := range v.fields {
			if h.Value != "" {
				count++
			}
		}
		ui.Textf(c, "已设置 %d 项", count).ContentTransition(numberChange).Grow(1).FontSize(13).FontWeight(580)
		if secondaryAction(c, "从剪贴板导入").Height(26).Label("从剪贴板导入请求头").Clicked() {
			block, err := application.ImportHeaders(c.ReadClipboard())
			v.headerErr = err
			if err == nil && block != "" {
				v.loadHeaders(block)
			}
		}
		if secondaryAction(c, "添加请求头").Height(26).Label("添加自定义请求头").Clicked() {
			v.fields = append(v.fields, requestField{})
			added = true
		}
	})
	inlineError(c, v.headerErr)
	remove := -1
	for i := range v.fields {
		h := &v.fields[i]
		if i < 4 {
			formRow(c, p, h.Name, func() { rowInput(c, p, h.Name, "", &h.Value, i < 2) })
			continue
		}
		ui.Row(c.Key("custom-" + strconv.Itoa(i))).Transition(pageFade).FillWidth().Height(32).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
			name := rowInput(c, p, "请求头名称", "名称", &h.Name, false).Grow(0).Basis(82).MinWidth(82)
			if added && i == len(v.fields)-1 {
				name.Focus().ScrollIntoView()
			}
			rowInput(c, p, "请求头值", "值", &h.Value, true)
			if iconButton(c, "close", "移除自定义请求头 "+strconv.Itoa(i-3), false) {
				remove = i
			}
		})
	}
	if remove >= 0 {
		v.fields = slices.Delete(v.fields, remove, remove+1)
	}
}
