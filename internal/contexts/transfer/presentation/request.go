package presentation

import (
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
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
func (v *View) requestDialog(c *ui.Context, p palette) {
	ui.DialogBase(c, &v.requestOpen, func(back, panel ui.Element) {
		panel.Width(360).Column().Padding(18).Gap(10).Radius(cornerSheet).Background(p.panel).Transition(panelMotion).Children(func() {
			ui.Text(c, "请求选项").FontSize(18).FontWeight(600)
			if v.fields == nil {
				v.loadHeaders("")
			}
			_, height := c.Size()
			ui.Scroll(c).FillWidth().Height(min(320, height-160)).Children(func() {
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					if secondaryAction(c, "单任务代理").Clicked() {
						v.openNetwork(true)
					}
					ui.Text(c, "备用下载地址").FontSize(12).TextColor(p.muted)
					ui.TextAreaBase(c, &v.mirrorText).Transition(controlMotion).FillWidth().Lines(2, 3).Padding(8).Radius(cornerSmall).Border(1, p.border).Label("备用下载地址").Placeholder("同一文件，每行一个备用地址")
					for i := range v.fields {
						h := &v.fields[i]
						if i < 4 {
							optionField(c, p, h.Name, &h.Value, i < 2)
						} else {
							ui.Row(c.Key(strconv.Itoa(i))).Transition(pageFade).Gap(5).FillWidth().Children(func() {
								for j, value := range []*string{&h.Name, &h.Value} {
									label := []string{"请求头名称", "请求头值"}[j]
									field := ui.TextInputBase(c, value).Transition(controlMotion).Grow(1).Height(30).PaddingX(6).Radius(cornerSmall).Border(1, p.border).Label(label).Placeholder(label)
									if j == 1 {
										field.Password()
									}
								}
							})
						}
					}
					inlineError(c, v.headerErr)
					if secondaryAction(c, "从剪贴板导入请求头").Clicked() {
						block, err := application.ImportHeaders(c.ReadClipboard())
						v.headerErr = err
						if err == nil && block != "" {
							v.loadHeaders(block)
						}
					}
					if secondaryAction(c, "添加自定义请求头").Clicked() {
						v.fields = append(v.fields, requestField{})
					}
				})
			})
			if primaryAction(c, "完成", "check", false).AlignSelf(ui.End).Clicked() {
				v.requestOpen = false
			}
		})
	})
}
