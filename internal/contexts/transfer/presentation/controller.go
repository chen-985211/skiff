package presentation

import (
	"context"
	"fmt"
	"skiff/internal/contexts/transfer/application"
	"strings"
	"time"
)

func (v *View) dispatch(fn func()) {
	if v.Actions.Dispatch != nil {
		v.Actions.Dispatch(fn)
	} else {
		fn()
	}
}
func (v *View) submit() {
	if strings.TrimSpace(v.URLs) == "" {
		v.FormError = "请先输入下载链接"
		return
	}
	if strings.TrimSpace(v.Directory) == "" {
		v.FormError = "请选择保存目录"
		return
	}
	if v.API == nil {
		return
	}
	v.Busy = true
	v.FormError = ""
	text, dir := v.URLs, v.Directory
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		n, err := v.API.Add(ctx, text, dir)
		snap := v.API.Snapshot()
		v.dispatch(func() {
			v.Busy = false
			v.Snapshot = snap
			if n > 0 {
				v.DialogOpen = false
				v.URLs, v.Directory = "", ""
				v.selectCategory("all")
				v.Query = ""
				v.Message = ""
				if err != nil {
					v.Message = fmt.Sprintf("已创建 %d 个任务，部分操作失败：%v", n, err)
				}
			} else if err != nil {
				v.FormError = err.Error()
			}
		})
	}()
}
func (v *View) command(id string, cmd application.Command) {
	v.run(id, func(ctx context.Context) error { return v.API.Execute(ctx, id, cmd) })
}
func (v *View) run(taskID string, action func(context.Context) error) {
	if v.Busy || v.API == nil {
		return
	}
	v.Busy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := action(ctx)
		snap := v.API.Snapshot()
		v.dispatch(func() {
			v.Busy = false
			v.Snapshot = snap
			if err != nil {
				v.Message = err.Error()
				for _, t := range v.filtered() {
					if t.ID == taskID && (v.detailID == "" || v.detailID == t.ID) && t.Error == v.Message {
						v.Message = ""
						break
					}
				}
			} else {
				v.Message = ""
			}
		})
	}()
}

func (v *View) selectedTheme() application.ThemeMode {
	if v.pendingTheme != nil {
		return *v.pendingTheme
	}
	return v.Snapshot.Preferences.Theme
}

// Show the latest choice immediately. Serialize writes so a slower previous
// choice can never overwrite the user's final one on disk.
func (v *View) chooseTheme(mode application.ThemeMode) {
	if v.API == nil || mode == v.selectedTheme() {
		return
	}
	if v.pendingTheme != nil {
		*v.pendingTheme = mode
		return
	}
	v.pendingTheme = &mode
	v.saveTheme(mode)
}

func (v *View) saveTheme(mode application.ThemeMode) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := v.API.SetTheme(ctx, mode)
		v.dispatch(func() {
			if err == nil {
				v.Snapshot.Preferences.Theme = mode
			}
			if latest := *v.pendingTheme; latest != mode {
				v.saveTheme(latest)
				return
			}
			v.pendingTheme = nil
			if err != nil {
				v.Message = err.Error()
			}
		})
	}()
}
