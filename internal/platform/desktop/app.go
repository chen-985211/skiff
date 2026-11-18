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
	"time"
)

func Run() error {
	dataDir, err := dataDirectory()
	if err != nil {
		return err
	}
	mygo.App.SetName("Skiff")
	mygo.App.SetLanguages("zh-CN")
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
							err = errors.New("所选路径不是文件夹")
						}
					}
					if err == nil {
						err = mygo.Shell.OpenPath(path)
					}
					if err != nil {
						err = fmt.Errorf("无法打开保存目录：%w", err)
					}
					win.Update(func() { done(err) })
				}()
			},
			Dispatch: func(fn func()) { win.Update(fn) },
			PickDirectory: func(current string, done func(string, error)) {
				go func() {
					paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "选择下载保存目录", DefaultPath: current, Directory: true, CreateDirectories: true, ButtonLabel: "选择文件夹"})
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
						return errors.New("文件或文件夹不存在")
					}
					return err
				}
				mygo.Shell.ShowItemInFolder(path)
				return nil
			},
		}
		view = presentation.New(nil, actions)
		win = mygo.NewWindow(mygo.WindowOptions{Title: "Skiff", TitleBarStyle: mygo.TitleBarHiddenInset, Width: 1020, Height: 680, MinWidth: 800, MinHeight: 560, StateKey: "skiff-main", Content: ui.View(view.Render)})
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Label: "文件", Submenu: []*mygo.MenuItem{{Label: "新建下载…", Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) { view.OpenNew(); win.Invalidate() }}}},
			{Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu},
		}))
		go func() {
			binary, err := application.PrepareEngine(pollCtx, aria2.NewInstallation(dataDir), installRequests, func(reason error) {
				win.Update(func() { view.OfferEngineDownload(reason) })
			})
			ctx, cancel := context.WithTimeout(pollCtx, 12*time.Second)
			if err == nil {
				process, err = aria2.Start(ctx, dataDir, binary)
			}
			if err == nil {
				service = application.New(process.Client, persistence.Repository{Path: filepath.Join(dataDir, "tasks.json")}, defaultDirectory, newID)
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
		panic(errors.New("无法生成下载任务 ID"))
	}
	return hex.EncodeToString(b[:])
}
