package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"skiff/internal/contexts/transfer/presentation"
	"skiff/internal/contexts/transfer/presentation/i18n"
	"time"
)

func Run() error {
	dataDir, err := dataDirectory()
	if err != nil {
		return err
	}
	mygo.App.SetName(i18n.Text(i18n.Source, "common.appName"))
	mygo.App.SetPath(mygo.PathUserData, dataDir)
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	var win *mygo.Window
	var view *presentation.View
	var service *application.Service
	var process *aria2.Process
	initialized := make(chan struct{})
	pollCtx, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()
	closing, closed := false, false
	mygo.App.OnSecondInstance(func([]string, string) {
		if win != nil && !closing {
			win.Show()
			win.Focus()
		}
	})
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if closed {
			return
		}
		e.PreventDefault()
		if closing {
			return
		}
		closing = true
		cancelPoll()
		if win != nil {
			win.Hide()
		}
		go func() {
			<-initialized
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if service != nil {
				if err := service.Flush(ctx); err != nil {
					log.Print(err)
				}
			}
			if process != nil {
				if err := process.Close(ctx); err != nil {
					log.Print(err)
				}
			}
			mygo.RunOnMain(func() { closed = true; mygo.App.Quit() })
		}()
	})
	mygo.App.OnWindowAllClosed(func() { mygo.App.Quit() })
	mygo.App.WhenReady(func() {
		downloads, _ := mygo.App.Path(mygo.PathDownloads)
		defaultDirectory := filepath.Join(downloads, "Skiff")
		installRequests := make(chan struct{}, 1)
		actions := presentation.Actions{
			Context:      pollCtx,
			ChosenLocale: mygo.App.ChosenLocale,
			SetLocale:    mygo.App.SetLocale,
			PickSource: func(folder bool, done func(string, error)) {
				go func() {
					paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: i18n.Text(mygo.App.Locale(), "bt.chooseSource"), Directory: folder})
					win.Update(func() {
						if len(paths) > 0 {
							done(paths[0], err)
						} else {
							done("", err)
						}
					})
				}()
			},
			SaveTorrent: func(path string, done func(string, error)) {
				go func() {
					out, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: i18n.Text(mygo.App.Locale(), "native.saveTorrent"), DefaultPath: path, CreateDirectories: true, Filters: []mygo.FileFilter{{Name: i18n.Text(mygo.App.Locale(), "native.torrentFilter"), Extensions: []string{"torrent"}}}})
					win.Update(func() { done(out, err) })
				}()
			},
			PickTorrent: func(done func(string, error)) {
				go func() {
					paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: i18n.Text(mygo.App.Locale(), "bt.open"), Filters: []mygo.FileFilter{{Name: i18n.Text(mygo.App.Locale(), "native.torrentFilter"), Extensions: []string{"torrent"}}}})
					win.Update(func() {
						if err != nil {
							done("", err)
						} else if len(paths) > 0 {
							done(paths[0], nil)
						}
					})
				}()
			},
			DownloadEngine: func() { installRequests <- struct{}{} },
			OpenDirectory: func(path string, done func(error)) {
				go func() {
					var err error
					if path == defaultDirectory {
						err = os.MkdirAll(path, 0755)
					}
					if err == nil {
						var info os.FileInfo
						info, err = os.Stat(path)
						if err == nil && !info.IsDir() {
							err = errors.New("path is not a directory")
						}
					}
					if err == nil {
						err = mygo.Shell.OpenPath(path)
					}
					if err != nil {
						err = fmt.Errorf("open download directory: %w", err)
					}
					win.Update(func() { done(err) })
				}()
			},
			Dispatch: func(fn func()) { win.Update(fn) },
			PickDirectory: func(current string, done func(string, error)) {
				go func() {
					paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: i18n.Text(mygo.App.Locale(), "native.pickDirectory"), DefaultPath: current, Directory: true, CreateDirectories: true, ButtonLabel: i18n.Text(mygo.App.Locale(), "native.pickFolder")})
					win.Update(func() {
						if err != nil {
							done("", err)
						} else if len(paths) > 0 {
							done(paths[0], nil)
						}
					})
				}()
			},
			Reveal: func(path string) error {
				if _, err := os.Stat(path); err != nil {
					if os.IsNotExist(err) {
						return errors.New("file or directory not found")
					}
					return err
				}
				mygo.Shell.ShowItemInFolder(path)
				return nil
			},
		}
		view = presentation.New(nil, actions)
		win = mygo.NewWindow(mygo.WindowOptions{Title: i18n.Text(mygo.App.Locale(), "common.appName"), TitleBarStyle: mygo.TitleBarHiddenInset, Width: 1020, Height: 680, MinWidth: 800, MinHeight: 560, StateKey: "skiff-main", Content: ui.View(view.Render)})
		setMenu := func(locale string) {
			mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
				{Role: mygo.RoleAppMenu},
				{Label: i18n.Text(locale, "common.file"), Submenu: []*mygo.MenuItem{{Label: i18n.Text(locale, "native.newDownload"), Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) { view.OpenNew(); win.Invalidate() }}}},
				{Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu},
			}))
		}
		setMenu(mygo.App.Locale())
		mygo.App.OnLocaleChanged(setMenu)
		go func() {
			binary, err := application.PrepareEngine(pollCtx, aria2.NewInstallation(dataDir), installRequests, func(reason error) {
				win.Update(func() { view.OfferEngineDownload(reason) })
			})
			ctx, cancel := context.WithTimeout(pollCtx, 12*time.Second)
			repo := persistence.Repository{Path: filepath.Join(dataDir, "tasks.json")}
			if err == nil {
				var state application.State
				state, err = repo.Load(ctx)
				if err == nil {
					process, err = aria2.StartConfigured(ctx, dataDir, binary, state.Preferences)
				}
			}
			if err == nil {
				service = application.New(process.Client, repo, defaultDirectory, newID)
				err = service.Open(ctx)
				if err != nil {
					// Quit must never flush a partially restored queue.
					service = nil
				}
			}
			cancel()
			close(initialized)
			if err != nil {
				if pollCtx.Err() == nil {
					win.Update(func() { view.FinishEngineStartup(nil, application.Snapshot{}, err) })
				}
				return
			}
			initial := service.Snapshot()
			win.Update(func() { view.FinishEngineStartup(service, initial, nil) })
			go func() {
				timer := time.NewTicker(time.Minute)
				defer timer.Stop()
				for {
					if err := service.RefreshBTSubscriptions(pollCtx, false); err != nil && pollCtx.Err() == nil {
						log.Print(err)
					}
					select {
					case <-pollCtx.Done():
						return
					case <-timer.C:
					}
				}
			}()
			previous := initial
			ticker := time.NewTicker(700 * time.Millisecond)
			defer ticker.Stop()
			hadError := false
			for {
				select {
				case <-pollCtx.Done():
					return
				case <-ticker.C:
				}
				ctx, cancel := context.WithTimeout(pollCtx, 5*time.Second)
				err := service.Refresh(ctx)
				cancel()
				if err != nil {
					if pollCtx.Err() != nil {
						return
					}
					if !hadError {
						win.Update(func() { view.EngineError = err })
					}
					hadError = true
					continue
				}
				current := service.Snapshot()
				if !reflect.DeepEqual(previous, current) || hadError {
					recovered := hadError
					win.Update(func() {
						view.Snapshot = current
						if recovered {
							view.EngineError = nil
						}
					})
					previous = current
					hadError = false
				}
			}
		}()
	})
	return mygo.App.Run()
}
func dataDirectory() (string, error) {
	if value := os.Getenv("SKIFF_DATA_DIR"); value != "" {
		return filepath.Abs(value)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Skiff"), nil
}
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("generate task ID: %w", err))
	}
	return hex.EncodeToString(b[:])
}
