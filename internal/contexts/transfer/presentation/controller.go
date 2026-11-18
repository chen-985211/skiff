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

// Each preference applies on input and serializes its own writes. The UI
// thread owns pending and the snapshot; the worker only sees the submitted value.
type preferenceSave[T comparable] struct {
	pending *T
}

func (p *preferenceSave[T]) current(saved T) T {
	if p.pending != nil {
		return *p.pending
	}
	return saved
}

func (p *preferenceSave[T]) choose(v *View, saved *T, choice T, write func(context.Context, T) error) {
	if p.current(*saved) == choice {
		return
	}
	if p.pending != nil {
		*p.pending = choice
		return
	}
	// The in-flight value stays immutable even if a later click retargets pending.
	submitted := choice
	p.pending = &choice
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := write(ctx, submitted)
		v.dispatch(func() {
			latest := *p.pending
			p.pending = nil
			if err == nil {
				*saved = submitted
			}
			if latest != submitted {
				p.choose(v, saved, latest, write)
			} else if err != nil {
				v.Message = err.Error()
			}
		})
	}()
}

func (v *View) selectedTheme() application.ThemeMode {
	return v.themeSave.current(v.Snapshot.Preferences.Theme)
}

func (v *View) staticBackground() bool {
	return v.flowSave.current(v.Snapshot.Preferences.StaticBackground)
}
