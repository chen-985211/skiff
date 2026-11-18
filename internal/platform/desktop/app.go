package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	mygo.App.SetName("轻舟")
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
		if win != nil {
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
		if view != nil {
			view.Busy = true
			view.Message = "正在保存下载任务…"
			win.Invalidate()
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
		actions := presentation.Actions{
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
			Reveal: func(path string) {
				if _, err := os.Stat(path); err != nil {
					view.Message = "文件或文件夹不存在"
					win.Invalidate()
					return
				}
				mygo.Shell.ShowItemInFolder(path)
			},
		}
		view = presentation.New(nil, actions)
		win = mygo.NewWindow(mygo.WindowOptions{Title: "轻舟", TitleBarStyle: mygo.TitleBarHiddenInset, Width: 1020, Height: 680, MinWidth: 800, MinHeight: 560, StateKey: "skiff-main", Content: ui.View(view.Render)})
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Label: "文件", Submenu: []*mygo.MenuItem{{Label: "新建下载…", Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) { view.OpenNew(); win.Invalidate() }}}},
			{Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu},
		}))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			binary, err := aria2.FindBinary()
			if err == nil {
				process, err = aria2.Start(ctx, dataDir, binary)
			}
			if err == nil {
				target, _ := mygo.App.Path(mygo.PathDownloads)
				service = application.New(process.Client, persistence.Repository{Path: filepath.Join(dataDir, "tasks.json")}, filepath.Join(target, "Skiff"), newID)
				err = service.Open(ctx)
			}
			close(initialized)
			if err != nil {
				win.Update(func() { view.Message = err.Error() })
				return
			}
			initial := service.Snapshot()
			win.Update(func() { view.API = service; view.Snapshot = initial; view.Ready = true })
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
						win.Update(func() { view.Message = err.Error() })
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
							view.Message = ""
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
