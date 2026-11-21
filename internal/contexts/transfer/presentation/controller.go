package presentation

import (
	"context"
	"fmt"
	"skiff/internal/contexts/transfer/application"
	"strconv"
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
	limit := 0
	if value := strings.TrimSpace(v.connectionLimit); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > application.MaxConnectionLimit {
			v.FormError = fmt.Sprintf("连接上限应为 1 到 %d，留空使用自动设置", application.MaxConnectionLimit)
			v.advanced = true
			return
		}
	}
	v.Busy = true
	v.FormError = ""
	text, dir := v.URLs, v.Directory
	opts := application.AddOptions{ConnectionLimit: limit, Network: v.taskNetwork, Mirrors: v.mirrorText, Headers: v.headerBlock(), OutputName: v.outputName}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := v.API.AddWithOptions(ctx, text, dir, opts)
		snap := v.API.Snapshot()
		v.dispatch(func() {
			v.Busy = false
			v.Snapshot = snap
			if err != nil && result.Added == 0 {
				v.FormError = err.Error()
				return
			}
			v.URLs = strings.Join(result.Remaining, "\n")
			v.selectCategory("all")
			v.Query = ""
			summary := fmt.Sprintf("新增 %d · 重复 %d · 无效 %d", result.Added, result.Duplicates, result.Invalid)
			if result.Deferred > 0 {
				summary += fmt.Sprintf(" · 待下批 %d", result.Deferred)
			}
			switch {
			case err != nil:
				v.FormError = fmt.Sprintf("已创建 %d 个任务，部分操作失败：%v", result.Added, err)
				if len(result.Remaining) > 0 || result.Duplicates > 0 {
					v.FormError = summary + "\n" + v.FormError
				}
			case len(result.Remaining) > 0:
				v.FormError = summary
				if result.Invalid > 0 {
					v.FormError += "。请修改无效的文件链接。"
				}
			default:
				v.DialogOpen = false
				v.Directory = ""
				v.advanced = false
				v.connectionLimit = ""
				v.outputName = ""
				v.fields = nil
				v.importedURL = ""
				v.mirrorText = ""
				v.taskNetwork = application.NetworkSettings{}
				if result.Added+result.Duplicates > 1 || result.Duplicates > 0 {
					v.batchNotice = summary
				}
			}
		})
	}()
}
func (v *View) command(id string, cmd application.Command) {
	v.queueError = nil
	v.run(func(ctx context.Context) error { return v.API.Execute(ctx, id, cmd) }, func(err error) {
		v.setTaskError(id, err)
	})
}

func (v *View) setTaskError(id string, err error) {
	delete(v.taskErrors, id)
	if err == nil {
		return
	}
	for _, task := range v.Snapshot.Tasks {
		if task.ID != id {
			continue
		}
		if task.Error != err.Error() {
			v.taskErrors[id] = err
		}
		return
	}
	// A remove can take effect in the engine before saving the queue fails.
	v.queueError = err
}

func (v *View) run(action func(context.Context) error, done func(error)) {
	if v.Busy || v.API == nil {
		return
	}
	v.Busy = true
	parent := v.Actions.Context
	if parent == nil {
		parent = context.Background()
	}
	go func() {
		err := action(parent)
		snap := v.API.Snapshot()
		v.dispatch(func() {
			v.Busy = false
			v.Snapshot = snap
			done(err)
		})
	}()
}

// Each preference applies on input and serializes its own writes. The UI
// thread owns pending and the snapshot; the worker only sees the submitted value.
type preferenceSave[T comparable] struct {
	pending *T
	err     error
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
	p.err = nil
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
				p.err = err
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
