package presentation

import (
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation/i18n"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	formInsetX          = 20
	formInsetY          = 14
	formGap             = 10
	formHeaderHeight    = 32
	formFooterHeight    = primaryActionHeight
	formErrorMaxHeight  = 68
	sourceMinHeight     = 104
	composePrimaryWidth = 132
	composeToolWidth    = 100
)

func (v *View) newDialog(c *ui.Context, p palette, anchor ui.Rect, reveal float32) {
	windowW, windowH := c.Size()
	const left = sidebarWidth + workspaceInset
	width := min(float32(420), windowW-left-workspaceInset)
	if anchor.W == 0 {
		// A menu or keyboard shortcut can open the form from Settings, without a dock.
		anchor = ui.Rect{
			X: left + min(dockWidth, width) - dockActionWidth,
			Y: windowH - footerInset - dockHeight,
			W: dockActionWidth,
			H: primaryActionHeight,
		}
	}
	closed := 1 - reveal
	radius := cornerPrimary + (cornerSheet-cornerPrimary)*reveal
	shown := true // Keep the modal and its draft mounted until the spring reaches the button.
	ui.DialogBase(c, &shown, func(back, panel ui.Element) {
		name := tr(c, "download.new")
		if v.advanced {
			name = tr(c, "download.options")
		}
		panel.Label(name).Absolute().Left(left+(anchor.X-left)*closed).Bottom(footerInset+(windowH-anchor.Y-anchor.H-footerInset)*closed).
			Width(anchor.W+(width-anchor.W)*reveal).Column().AlignItems(ui.Start).Radius(radius).Clip().
			Shadow(0, 6*reveal, 20*reveal, 0, p.scrim.Alpha(.15*reveal))

		buttonInk := max(0, 1-reveal/.35)
		// The glass rim belongs to the paint layer. Including it in layout would
		// offset the button label and add its thickness again at the open endpoint.
		glass := ui.Box(c.Key("sheet-glass")).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			ZIndex(-1).Opacity(1 - buttonInk).IgnoreInput(true).AccessHidden(true)
		surface(c, glass, p, radius, sheetFrost)
		ui.Box(c.Key("button-fill")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Radius(radius).
			Background(p.accent.Alpha(buttonInk)).IgnoreInput(true)
		fields := min(max((reveal-.48)/.42, 0), 1)
		// Measure the clipped form at its reading width throughout the transition.
		form := ui.Column(c.Key("form")).Width(width).Shrink(0).Padding(formInsetY, formInsetX).Gap(formGap).
			Opacity(fields).IgnoreInput(!v.DialogOpen).AccessHidden(!v.DialogOpen)
		advanced := form.AnimateTo("advanced", amount(v.advanced), panelSpring)
		ink := form.AnimateTo("editor-ink", amount(v.advanced), colorFade)
		btInk := form.AnimateTo("bt-source-ink", amount(v.bt.tab == 1), colorFade)
		form.Children(func() {
			toggle := v.composeTools(c, p, width-2*formInsetX, ink, btInk, v.Busy || fields < .5)
			back.Background(p.canvas.Alpha(.18 * advanced * reveal))
			v.composeFields(c, p, width-2*formInsetX, advanced, ink, btInk)
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
			title := ui.Row(c.Key("compose-title")).Transition(controlMotion).Absolute().Left(0).Top(0).
				Size(anchor.W, anchor.H).PaddingX(dockActionInset).Opacity(buttonInk).IgnoreInput(true).AccessHidden(true)
			primaryActionContent(c, title, tr(c, "download.new"), "plus")
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

func (v *View) composeTools(c *ui.Context, p palette, width, ink, btInk float32, disabled bool) ui.Element {
	actionWidth := float32(64)
	if c.Locale() == "en" {
		actionWidth = 88
	}
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
				v.optionToolbar(c, p, min(264, width-composeToolWidth-gap), ink)
			}
			count := 0
			for line := range strings.Lines(v.URLs) {
				if strings.TrimSpace(line) != "" {
					count++
				}
			}
			if count > 1 && btInk < 1 {
				ui.Text(c, trf(c, "download.lines", i18n.Args{"count": count})).Transition(pageFade).Absolute().Right(0).Top(8).
					Opacity(editorOpacity(1-ink) * editorOpacity(1-btInk)).AccessHidden(v.advanced || v.bt.tab == 1).IgnoreInput(true).FontSize(11).TextColor(p.muted).ContentTransition(numberChange)
			}
		})
		tools := ui.Box(c.Key("source-tools")).Width(composeToolWidth).FillHeight()
		tools.Children(func() {
			link := ui.Box(c.Key("link-tools")).Absolute().Right(0).Top(0).Bottom(0).
				Opacity(editorOpacity(1 - btInk)).IgnoreInput(v.bt.tab == 1).AccessHidden(v.bt.tab == 1)
			link.Children(func() { toggle = v.advancedToggle(c, p, actionWidth, ink, disabled) })
			if v.bt.tab == 1 || btInk > 0 {
				bt := ui.Box(c.Key("bt-tools")).Absolute().Right(0).Top(0).Bottom(0).
					Opacity(editorOpacity(btInk)).IgnoreInput(v.bt.tab != 1).AccessHidden(v.bt.tab != 1)
				bt.Children(func() { v.torrentTools(c, p) })
			}
		})
	})
	return toggle
}

func (v *View) advancedToggle(c *ui.Context, p palette, actionWidth, ink float32, disabled bool) ui.Element {
	action := tr(c, "download.advanced")
	if v.advanced {
		action = tr(c, "download.backToLinks")
	}
	toggle := ui.ButtonBase(c.Key("advanced-toggle")).Transition(controlMotion).Label(action).Tooltip(action).Expanded(v.advanced).
		Width(actionWidth).FillHeight().Radius(cornerSmall).FocusRing(true).Disabled(disabled)
	if toggle.Hovered() {
		toggle.Background(p.panel.Alpha(.5))
	}
	toggle.Row().Center().Gap(6).Children(func() {
		ui.Box(c).Size(15, 15).Center().AccessHidden(true).Children(func() {
			ui.Icon(c, icons["settings"]).FontSize(15).TextColor(p.muted).Opacity(editorOpacity(1 - ink))
			ui.Icon(c, icons["chevron"]).Absolute().Left(0).Top(0).FontSize(15).Rotate(90).TextColor(p.muted).Opacity(editorOpacity(ink))
		})
		composeCaption(c, tr(c, "download.advancedShort"), tr(c, "common.back"), ink, 12, 550, p.muted)
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

// Measure pages at their reading width and interpolate the shared viewport.
// Keep the outgoing page available to paint while only the selected page takes input.
func sourceEditors(c *ui.Context, viewport ui.Element, second bool, ink float32, firstPage, secondPage func()) {
	size := viewport.AnimateTo("page-height", amount(second), panelSpring)
	viewport.Children(func() {
		var heights [2]float32
		for i, render := range []func(){firstPage, secondPage} {
			active := (i == 1) == second
			weight, extent := pageWeight(i, ink), pageWeight(i, size)
			if !active && weight == 0 && extent == 0 {
				continue
			}
			page := ui.Column(c.Key(i)).FillWidth().Shrink(0).Opacity(editorOpacity(weight)).IgnoreInput(!active).AccessHidden(!active)
			if !active {
				page.Absolute().Top(0).Left(0).Right(0)
			}
			page.Children(render)
			heights[i] = max(float32(sourceMinHeight), page.LayoutBounds().H)
		}
		if size != amount(second) {
			viewport.Height(heights[0] + (heights[1]-heights[0])*size)
		}
	})
}

func (v *View) composeFields(c *ui.Context, p palette, width, advanced, ink, btInk float32) {
	p.muted = p.muted.Mix(c.Theme().Text, .38)
	_, windowH := c.Size()
	const chrome = 2*formInsetY + formHeaderHeight + 2*formGap + formFooterHeight + footerInset
	available := windowH - max(float32(36), c.TitleBar().Bottom+8) - chrome
	if v.FormError != nil {
		available -= formErrorMaxHeight + formGap
	}
	optionsHeight := min(float32(194), available)
	ui.Column(c.Key("editor-viewport")).Width(width + optionGutter).Clip().Children(func() {
		source := ui.Column(c.Key("source-fields")).Width(width).Shrink(0).
			Opacity(editorOpacity(1 - ink)).IgnoreInput(v.advanced).AccessHidden(v.advanced)
		sourceEditors(c, source, v.bt.tab == 1, btInk,
			func() { v.linkEditor(c, p) }, func() { v.torrentEditor(c, p) })
		// The link editor measures wrapped text at its full width, including
		// while it is offscreen. Only the difference in page heights is added.
		linkHeight := max(float32(sourceMinHeight), source.LayoutBounds().H)
		ui.Box(c).Height(max(0, optionsHeight-linkHeight) * advanced).Shrink(0)
		if ink > 0 || v.advanced {
			v.advancedOptions(c, p, width, optionsHeight, ink)
		}
	})
	if message := errorText(c.Locale(), v.FormError); message != "" {
		ui.Text(c, message).Transition(pageFade).ContentTransition(textChange).MaxHeight(formErrorMaxHeight).Tooltip(message).FontSize(12).TextColor(c.Theme().Danger)
	}
	v.composeFooter(c, p)
}

func (v *View) composeFooter(c *ui.Context, p palette) {
	footer := ui.Box(c.Key("compose-footer")).FillWidth().Height(formFooterHeight)
	creating := v.bt.tab == 1 && v.bt.creator.open
	ink := footer.AnimateTo("creator", amount(creating), colorFade)
	width := primaryActionWidth(c, composePrimaryWidth, tr(c, "download.start"), tr(c, "bt.readFiles"),
		tr(c, "bt.cancelFetch"), tr(c, "bt.save"), tr(c, "bt.cancelCreate"))
	footer.Children(func() {
		for i, render := range []func(*ui.Context, palette, float32){v.downloadFooter, v.createFooter} {
			active := (i == 1) == creating
			weight := pageWeight(i, ink)
			if !active && weight == 0 {
				continue
			}
			row := ui.Row(c.Key(i)).Absolute().Left(0).Right(0).Top(0).Bottom(0).AlignItems(ui.Center).Gap(12).
				Opacity(editorOpacity(weight)).IgnoreInput(!active).AccessHidden(!active)
			row.Children(func() { render(c, p, width) })
		}
	})
}

func (v *View) downloadFooter(c *ui.Context, p palette, width float32) {
	v.composeDestination(c, p)
	caption := tr(c, "download.start")
	if v.bt.tab == 1 && v.bt.meta.Hash == "" {
		caption = tr(c, "bt.readFiles")
		if v.bt.cancel != nil {
			caption = tr(c, "bt.cancelFetch")
		}
	}
	if primaryAction(c, caption, "download", v.Busy).Width(width).Shrink(0).FocusRing(true).Clicked() {
		v.submit()
	}
}

func (v *View) linkEditor(c *ui.Context, p palette) {
	links := ui.TextAreaBase(c, &v.URLs).Transition(controlMotion).FillWidth().Lines(2, 7).MinHeight(sourceMinHeight).Padding(12, 14).FontSize(13).FixedLineHeight(20).
		Radius(cornerField).Label(tr(c, "download.urls")).Placeholder(tr(c, "download.paste")).PlaceholderColor(p.muted).
		Description(tr(c, "download.protocols")).Disabled(v.Busy)
	if v.focusLinks && !v.advanced && v.bt.tab == 0 {
		links.Focus()
		v.focusLinks = false
	}
	styleSourceField(c, links, p)
	if links.Changed() {
		v.FormError = nil
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

func styleSourceField(c *ui.Context, field ui.Element, p palette) {
	edge, focusEdge := p.border, p.active
	if !c.Preferences().HighContrast {
		edge, focusEdge = p.muted.Alpha(.36), p.active.Alpha(.65)
	}
	if field.Focused() {
		edge = focusEdge
	}
	field.Border(1, edge)
}

func (v *View) composeDestination(c *ui.Context, p palette) {
	ui.Row(c).Grow(1).MinWidth(0).Height(formFooterHeight).AlignItems(ui.Center).Gap(2).Children(func() {
		pick := func() {
			v.Actions.PickDirectory(v.Directory, func(path string, err error) {
				if err != nil {
					v.FormError = err
				} else if path != "" {
					v.Directory, v.FormError = path, nil
				}
			})
		}
		if v.Snapshot.Preferences.RecentDirectories[0] != "" {
			dropdown(c.Key("recent-destinations"), p, tr(c, "download.chooseDirectory"), "", v.Busy, func(m *ui.Menu) {
				w, _ := c.Size()
				m.MaxWidth(min(400, w-24))
				if v.Actions.PickDirectory != nil && m.Item(tr(c, "common.chooseFolder")).Chosen() {
					pick()
				}
				for _, dir := range v.Snapshot.Preferences.RecentDirectories {
					if dir != "" && m.Item(dir).Tooltip(dir).Checked(dir == v.Directory).Chosen() {
						v.Directory, v.FormError = dir, nil
					}
				}
			}, icons["folder"]).Tooltip(tr(c, "download.folderOrRecent"))
		} else {
			var label ui.Element
			if v.Actions.PickDirectory != nil {
				label = ui.ButtonBase(c.Key("pick-directory")).Transition(controlMotion).Label(tr(c, "download.chooseDirectory")).Tooltip(tr(c, "download.chooseDirectory")).
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
				Radius(cornerInline).FontSize(12).Label(tr(c, "download.folder")).Tooltip(v.Directory).Disabled(v.Busy)
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
				v.FormError = nil
			}
		})
	})
}
