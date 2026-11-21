package presentation

import (
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	formInsetX         = 20
	formInsetY         = 14
	formGap            = 10
	formHeaderHeight   = 32
	formFooterHeight   = 36
	formErrorMaxHeight = 68
)

func (v *View) newDialog(c *ui.Context, p palette, anchor ui.Rect, reveal float32) {
	windowW, windowH := c.Size()
	const left = sidebarWidth + workspaceInset
	width := min(float32(420), windowW-left-workspaceInset)
	if anchor.W == 0 {
		// A menu or keyboard shortcut can open the form from Settings, without a dock.
		anchor = ui.Rect{X: left + min(cardWidth, width) - composeButtonWidth, Y: windowH - footerInset - dockHeight, W: composeButtonWidth, H: dockHeight}
	}
	closed := 1 - reveal
	shown := true // Keep the modal and its draft mounted until the spring reaches the button.
	ui.DialogBase(c, &shown, func(back, panel ui.Element) {
		name := "新建下载"
		if v.advanced {
			name = "下载选项"
		}
		panel.Label(name).Absolute().Left(left+(anchor.X-left)*closed).Bottom(footerInset+(windowH-anchor.Y-anchor.H-footerInset)*closed).
			Width(anchor.W+(width-anchor.W)*reveal).Column().AlignItems(ui.Start).Radius(cornerSheet).Clip().
			Shadow(0, 6*reveal, 20*reveal, 0, p.scrim.Alpha(.15*reveal))

		buttonInk := max(0, 1-reveal/.35)
		// The glass rim belongs to the paint layer. Including it in layout would
		// offset the button label and add its thickness again at the open endpoint.
		glass := ui.Box(c.Key("sheet-glass")).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			ZIndex(-1).Opacity(1 - buttonInk).IgnoreInput(true).AccessHidden(true)
		surface(c, glass, p, cornerSheet, sheetFrost)
		ui.Box(c.Key("button-fill")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Radius(cornerSheet).
			Background(p.accent.Alpha(buttonInk)).IgnoreInput(true)
		fields := min(max((reveal-.48)/.42, 0), 1)
		// Measure the clipped form at its reading width throughout the transition.
		form := ui.Column(c.Key("form")).Width(width).Shrink(0).Padding(formInsetY, formInsetX).Gap(formGap).
			Opacity(fields).IgnoreInput(!v.DialogOpen).AccessHidden(!v.DialogOpen)
		advanced := form.AnimateTo("advanced", amount(v.advanced), panelSpring)
		ink := form.AnimateTo("editor-ink", amount(v.advanced), colorFade)
		form.Children(func() {
			toggle := v.composeTools(c, p, width-2*formInsetX, ink, v.Busy || fields < .5)
			back.Background(p.canvas.Alpha(.18 * advanced * reveal))
			v.composeFields(c, p, width-2*formInsetX, advanced, ink)
			if toggle.Clicked() {
				v.advanced = !v.advanced
				v.focusLinks = !v.advanced
				if v.advanced {
					toggle.Focus()
				}
				c.Invalidate()
			}
			if !shown && v.advanced && !back.Clicked() && !v.Busy {
				shown = true
				v.advanced = false
				toggle.Focus()
				c.Invalidate()
			}
		})
		if reveal < 1 {
			height := max(anchor.H, form.LayoutBounds().H)
			panel.Height(anchor.H + (height-anchor.H)*reveal)
		}

		// Carry the dock label through the start of the morph, then yield to the editor.
		if buttonInk > 0 {
			ui.Row(c.Key("compose-title")).Transition(controlMotion).Absolute().Left(17).Top(9).Height(26).
				Gap(7).AlignItems(ui.Center).Opacity(buttonInk).IgnoreInput(true).AccessHidden(true).Children(func() {
				ui.Icon(c, icons["plus"]).FontSize(15).TextColor(c.Theme().AccentText)
				ui.Text(c, "新建下载").SingleLine().FontSize(13).FontWeight(600).TextColor(c.Theme().AccentText)
			})
		}
		// Until the inputs are revealed, a second press belongs to the morphing
		// button. Keep keyboard focus in the editor so a paste is accepted immediately.
		if !v.Busy && (!v.DialogOpen || fields < .5) {
			hit := ui.Box(c.Key("reverse-compose")).Absolute().Left(0).Right(0).Top(0).Bottom(0).AccessHidden(true)
			if hit.Clicked() || !v.DialogOpen && back.OverlayShortcut(ui.Cmd, ui.KeyN) {
				if v.DialogOpen {
					v.DialogOpen = false
				} else {
					v.OpenNew()
				}
				c.Invalidate()
			}
		}
	})
	if !shown && !v.Busy {
		v.DialogOpen = false
		c.Invalidate()
	}
}

func (v *View) composeTools(c *ui.Context, p palette, width, ink float32, disabled bool) ui.Element {
	const actionWidth = 64
	const gap = 12
	header := ui.Row(c.Key("compose-tools")).FillWidth().Height(formHeaderHeight).AlignItems(ui.Center).Gap(gap)
	var toggle ui.Element
	header.Children(func() {
		leading := ui.Box(c).Grow(1).MinWidth(0).FillHeight()
		leading.Children(func() {
			if _, ok := v.API.(torrentAPI); ok && (!v.advanced || ink < 1) {
				v.sourceTabs(c, p, ink)
			}
			if v.advanced || ink > 0 {
				v.optionToolbar(c, p, min(264, width-actionWidth-gap), ink)
			}
			count := 0
			for line := range strings.Lines(v.URLs) {
				if strings.TrimSpace(line) != "" {
					count++
				}
			}
			if count > 1 && v.bt.tab == 0 {
				ui.Textf(c, "已输入 %d 行", count).Transition(pageFade).Absolute().Right(0).Top(8).
					Opacity(editorOpacity(1 - ink)).AccessHidden(v.advanced).FontSize(11).TextColor(p.muted).ContentTransition(numberChange)
			}
		})
		if v.bt.tab == 1 {
			v.btWait(c, p)
			return
		}
		action := "高级选项"
		if v.advanced {
			action = "返回链接"
		}
		toggle = ui.ButtonBase(c.Key("advanced-toggle")).Transition(controlMotion).Label(action).Tooltip(action).Expanded(v.advanced).
			Width(actionWidth).FillHeight().Radius(cornerSmall).FocusRing(true).Disabled(disabled)
		if toggle.Hovered() {
			toggle.Background(p.panel.Alpha(.5))
		}
		toggle.Row().Center().Gap(6).Children(func() {
			ui.Box(c).Size(15, 15).Center().AccessHidden(true).Children(func() {
				ui.Icon(c, icons["settings"]).FontSize(15).TextColor(p.muted).Opacity(editorOpacity(1 - ink))
				ui.Icon(c, icons["chevron"]).Absolute().Left(0).Top(0).FontSize(15).Rotate(90).TextColor(p.muted).Opacity(editorOpacity(ink))
			})
			composeCaption(c, "高级", "返回", ink, 12, 550, p.muted)
		})
	})
	return toggle
}

// Both captions are painted at the current position of the toolbar.
func composeCaption(c *ui.Context, first, second string, progress, size float32, weight int, ink ui.Color) {
	ui.Box(c).AccessHidden(true).Children(func() {
		ui.Text(c, first).SingleLine().FontSize(size).FontWeight(weight).TextColor(ink).Opacity(max(0, 1-2*progress))
		ui.Text(c, second).Absolute().Left(0).Top(0).SingleLine().FontSize(size).FontWeight(weight).TextColor(ink).Opacity(max(0, 2*progress-1))
	})
}

func (v *View) composeFields(c *ui.Context, p palette, width, advanced, ink float32) {
	p.muted = p.muted.Mix(c.Theme().Text, .38)
	_, windowH := c.Size()
	const chrome = 2*formInsetY + formHeaderHeight + 2*formGap + formFooterHeight + footerInset
	available := windowH - max(float32(36), c.TitleBar().Bottom+8) - chrome
	if v.FormError != "" {
		available -= formErrorMaxHeight + formGap
	}
	optionsHeight := min(float32(194), available)
	ui.Column(c.Key("editor-viewport")).Width(width + optionGutter).Clip().Children(func() {
		source := ui.Column(c.Key("source-fields")).Width(width).Shrink(0).
			Opacity(editorOpacity(1 - ink)).IgnoreInput(v.advanced).AccessHidden(v.advanced)
		source.Children(func() {
			if v.bt.tab == 1 {
				v.torrentEditor(c, p)
			} else {
				v.linkEditor(c, p)
			}
		})
		// The link editor measures wrapped text at its full width, including
		// while it is offscreen. Only the difference in page heights is added.
		linkHeight := max(float32(104), source.LayoutBounds().H)
		ui.Box(c).Height(max(0, optionsHeight-linkHeight) * advanced).Shrink(0)
		if ink > 0 || v.advanced {
			v.advancedOptions(c, p, width, optionsHeight, ink)
		}
	})
	if v.FormError != "" {
		ui.Text(c, v.FormError).Transition(pageFade).ContentTransition(textChange).MaxHeight(formErrorMaxHeight).Tooltip(v.FormError).FontSize(12).TextColor(c.Theme().Danger)
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		v.composeDestination(c, p)
		caption := "开始下载"
		if v.bt.tab == 1 && v.bt.meta.Hash == "" {
			caption = "读取文件列表"
			if v.bt.cancel != nil {
				caption = "取消获取"
			}
		}
		if primaryAction(c, caption, "download", v.Busy).Width(106).Shrink(0).FocusRing(true).Clicked() {
			v.submit()
		}
	})
}

func (v *View) linkEditor(c *ui.Context, p palette) {
	links := ui.TextAreaBase(c, &v.URLs).Transition(controlMotion).FillWidth().Lines(2, 7).MinHeight(104).Padding(12, 14).FontSize(13).FixedLineHeight(20).
		Radius(cornerField).Label("下载链接").Placeholder("粘贴下载链接或 cURL").PlaceholderColor(p.muted).
		Description("支持 HTTP、HTTPS 和 FTP 文件链接").Disabled(v.Busy)
	if !v.advanced {
		links.AutoFocus()
		if v.focusLinks {
			links.Focus()
			v.focusLinks = false
		}
	}
	edge, focusEdge := p.border, p.active
	if !c.Preferences().HighContrast {
		edge, focusEdge = p.muted.Alpha(.36), p.active.Alpha(.65)
	}
	if links.Focused() {
		edge = focusEdge
	}
	links.Border(1, edge)
	if links.Changed() {
		v.FormError = ""
		if v.importedURL != "" && v.URLs != v.importedURL {
			v.fields = nil
			v.importedURL = ""
		}
		if request, ok, err := application.ImportCurl(v.URLs); ok && err == nil {
			v.URLs = request.URL
			v.importedURL = request.URL
			v.loadHeaders(request.Headers)
			c.Invalidate()
		}
	}
}

func (v *View) composeDestination(c *ui.Context, p palette) {
	ui.Row(c).Grow(1).MinWidth(0).Height(formFooterHeight).AlignItems(ui.Center).Gap(2).Children(func() {
		pick := func() {
			v.Actions.PickDirectory(v.Directory, func(path string, err error) {
				if err != nil {
					v.FormError = err.Error()
				} else if path != "" {
					v.Directory, v.FormError = path, ""
				}
			})
		}
		if v.Snapshot.Preferences.RecentDirectories[0] != "" {
			dropdown(c.Key("recent-destinations"), p, "选择保存目录", "", v.Busy, func(m *ui.Menu) {
				w, _ := c.Size()
				m.MaxWidth(min(400, w-24))
				if v.Actions.PickDirectory != nil && m.Item("选择文件夹…").Chosen() {
					pick()
				}
				for _, dir := range v.Snapshot.Preferences.RecentDirectories {
					if dir != "" && m.Item(dir).Tooltip(dir).Checked(dir == v.Directory).Chosen() {
						v.Directory, v.FormError = dir, ""
					}
				}
			}, icons["folder"]).Tooltip("选择文件夹或最近位置")
		} else {
			var label ui.Element
			if v.Actions.PickDirectory != nil {
				label = ui.ButtonBase(c.Key("pick-directory")).Transition(controlMotion).Label("选择保存目录").Tooltip("选择保存目录").
					Radius(cornerInline).FocusRing(true).Disabled(v.Busy)
			} else {
				label = ui.Row(c)
			}
			if v.Actions.PickDirectory != nil && label.Hovered() {
				label.Background(p.panel.Alpha(.45))
			}
			label.Size(30, 32).Center().Children(func() {
				ui.Icon(c, icons["folder"]).FontSize(15).TextColor(p.muted)
			})
			if v.Actions.PickDirectory != nil && label.Clicked() {
				pick()
			}
		}
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Justify(ui.Center).Children(func() {
			dir := ui.TextInputBase(c, &v.Directory).Transition(controlMotion).Absolute().Left(0).Right(0).Top(0).Bottom(0).PaddingX(6).
				Radius(cornerInline).FontSize(12).Label("保存目录").Tooltip(v.Directory).Disabled(v.Busy)
			if dir.Focused() || c.Preferences().HighContrast {
				dir.Border(1, p.border)
			}
			caption := v.Directory
			if caption != "" && caption == v.Snapshot.Directory {
				caption = filepath.Base(caption)
			}
			if !dir.Focused() {
				dir.TextColor(ui.Transparent)
				ui.Text(c, caption).Transition(pageFade).Margin(0, 6).FontSize(12).TextColor(p.muted).SingleLine().TruncateMiddle().IgnoreInput(true).AccessHidden(true)
			}
			if dir.Changed() {
				v.FormError = ""
			}
		})
	})
}
