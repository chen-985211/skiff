package presentation

import (
	"context"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

type API interface {
	RetryFTP(context.Context, string, string, string) error
	SetConcurrency(context.Context, int) error
	SetDownloadLimit(context.Context, string, int64) error
	SetNetwork(context.Context, application.NetworkSettings) error
	SetDiskSettings(context.Context, application.DiskSettings) error
	SetTheme(context.Context, application.ThemeMode) error
	SetDefaultDirectory(context.Context, string) error
	SetStaticBackground(context.Context, bool) error
	AddWithOptions(context.Context, string, string, application.AddOptions) (application.BatchResult, error)
	Execute(context.Context, string, application.Command) error
	SetPaused(context.Context, []string, bool) error
	Snapshot() application.Snapshot
}
type Actions struct {
	ChosenLocale   func() string
	SetLocale      func(string)
	PickSource     func(bool, func(string, error))
	SaveTorrent    func(string, func(string, error))
	PickTorrent    func(func(string, error))
	Context        context.Context
	DownloadEngine func()
	OpenDirectory  func(string, func(error))
	Dispatch       func(func())
	PickDirectory  func(string, func(string, error))
	Reveal         func(string) error
}
type View struct {
	btNetwork                                      btNetworkPanel
	btInfo                                         btInspector
	bt                                             torrentDraft
	mirrorText                                     string
	importedURL                                    string
	networkOpen                                    bool
	networkDraft, taskNetwork                      application.NetworkSettings
	networkErr                                     error
	rateOpen                                       bool
	rateID, rateText                               string
	rateErr, globalRateErr                         error
	rateSaves                                      map[string]*preferenceSave[int64]
	diskOpen                                       bool
	cacheText, splitText, allocation               string
	diskErr                                        error
	headerErr                                      error
	optionTab                                      int
	fields                                         []requestField
	outputName                                     string
	ftpOpen                                        bool
	ftpID, ftpUser, ftpPassword                    string
	concurrencySave                                preferenceSave[int]
	concurrencyText                                string
	concurrencyEdited                              bool
	advanced, focusLinks                           bool
	connectionLimit                                string
	themeSave                                      preferenceSave[application.ThemeMode]
	flowSave                                       preferenceSave[bool]
	lastDefault                                    string
	batchNotice                                    messageError
	settingsOpen                                   bool
	light                                          windowLight
	API                                            API
	Actions                                        Actions
	Snapshot                                       application.Snapshot
	Ready, Busy, DialogOpen                        bool
	RestartOpen                                    bool
	RestartID                                      string
	Filter, Query, URLs, Directory                 string
	FormError                                      error
	EngineError                                    error
	engineOffered, installingEngine                bool
	taskErrors                                     map[string]error
	visibleTransfers                               map[string]visibleTransfer
	queueError, directoryError, openDirectoryError error
	detailID                                       string
	taskRows                                       ui.ListState
	gridColumns, detailColumn                      int
	detailTop                                      float32
}

func New(api API, actions Actions) *View {
	return &View{
		API: api, Actions: actions, Filter: "all", Ready: api != nil,
		rateSaves:  make(map[string]*preferenceSave[int64]),
		taskErrors: make(map[string]error), visibleTransfers: make(map[string]visibleTransfer),
	}
}
func (v *View) OfferEngineDownload(err error) {
	v.engineOffered = true
	v.installingEngine = false
	v.EngineError = err
}

func (v *View) FinishEngineStartup(api API, snapshot application.Snapshot, err error) {
	v.engineOffered, v.installingEngine = false, false
	v.API, v.Snapshot, v.EngineError = api, snapshot, err
	v.Ready = api != nil && err == nil
}

func (v *View) OpenNew() {
	if v.Ready && !v.Busy && !v.DialogOpen && !v.RestartOpen && !v.networkOpen && !v.diskOpen && !v.rateOpen && !v.ftpOpen && !v.btNetwork.open && !v.btInfo.open {
		v.DialogOpen = true
		v.focusLinks = v.bt.tab == 0 && !v.advanced
		if v.URLs == "" && (v.Directory == "" || v.Directory == v.lastDefault) {
			v.Directory = v.Snapshot.Directory
			v.lastDefault = v.Directory
		}
		v.FormError = nil
	}
}
func (v *View) Render(c *ui.Context) {
	defer v.forgetHiddenTransfers()
	if !v.DialogOpen {
		v.bt.stopLookup()
	}
	stage := ui.Row(c).Fill().AlignItems(ui.Stretch)
	mode := v.selectedTheme()
	dark := mode == application.ThemeDark || mode == application.ThemeSystem && c.Theme().Dark
	shade := stage.AnimateTo("theme", amount(dark), themeFade)
	if c.Preferences().HighContrast {
		shade = amount(dark)
	}
	p := appearance(c, shade)
	stage.Background(p.canvas).TextColor(c.Theme().Text)
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		v.OpenNew()
	}
	translucent := !c.Preferences().ReduceTransparency && !c.Preferences().HighContrast
	if translucent {
		still := c.Preferences().ReduceMotion || v.staticBackground()
		stage.Draw(func(painter *ui.Painter, r ui.Rect) { v.light.draw(painter, r, still, shade) })
	} else {
		v.light.moving = false
	}
	var search, composeButton ui.Element
	stage.Children(func() {
		v.sidebar(c, p)
		main := ui.Column(c).Grow(1).FillHeight().MinWidth(0).Padding(0, workspaceInset)
		main.Children(func() {
			ui.Box(c).Height(max(20, c.TitleBar().Bottom)).FillWidth().DragWindow()
			if v.settingsOpen {
				v.settingsPage(c, p)
			} else {
				ui.Column(c.Key("category-" + v.Filter)).Grow(1).MinHeight(0).Transition(pageFade).Children(func() {
					v.queue(c, p)
				})
				search, composeButton = v.dock(c, p)
			}
		})
	})
	// Keep the animation on the workspace, which is already on screen
	// before the sheet is built and survives dismissal or a quick reopen.
	reveal := stage.AnimateTo("compose", amount(v.DialogOpen), panelSpring)
	if v.DialogOpen || reveal > 0 {
		anchor := composeButton.Bounds()
		composeButton.Opacity(0)
		search.Opacity(max(0, 1-reveal/.35))
		v.newDialog(c, p, anchor, reveal)
	}
	v.renderBTNetwork(c, p, stage)
	v.renderBTDetails(c, p, stage)
	v.rateDialog(c, p, stage)
	v.diskDialog(c, p, stage)
	v.networkDialog(c, p, stage)
	v.ftpLogin(c, p, stage)
	if ui.AlertDialog(c, &v.RestartOpen, tr(c, "task.confirmRestart"), tr(c, "task.restartExplanation"), tr(c, "common.cancel"), tr(c, "task.restartConfirm")) == 1 {
		v.command(v.RestartID, application.Restart)
	}
}

func inlineError(c *ui.Context, err error) {
	inlineErrorMessage(c, errorText(c.Locale(), err))
}

func inlineErrorMessage(c *ui.Context, message string) {
	if message != "" {
		ui.Text(c.Key("inline-error"), message).Transition(pageFade).ContentTransition(textChange).MinWidth(0).MaxWidthPercent(100).SingleLine().FontSize(12).TextColor(c.Theme().Danger).Tooltip(message)
	}
}
