package presentation

import (
	"fmt"
	"strings"

	"skiff/internal/contexts/transfer/application"
)

func (v *View) filtered() []application.TaskDTO {
	var tasks []application.TaskDTO
	q := strings.ToLower(strings.TrimSpace(v.Query))
	for i := len(v.Snapshot.Tasks) - 1; i >= 0; i-- {
		t := v.Snapshot.Tasks[i]
		if matches(t, v.Filter) && (q == "" || strings.Contains(strings.ToLower(t.Name+" "+t.URL), q)) {
			tasks = append(tasks, t)
		}
	}
	return tasks
}
func matches(t application.TaskDTO, filter string) bool {
	return filter == "all" || t.Status == filter || (filter == "active" && t.Status == "waiting")
}
func (v *View) count(filter string) int {
	n := 0
	for _, t := range v.Snapshot.Tasks {
		if matches(t, filter) {
			n++
		}
	}
	return n
}
func statusText(status string) string {
	switch status {
	case "active":
		return "下载中"
	case "waiting":
		return "等待中"
	case "paused":
		return "已暂停"
	case "complete":
		return "已完成"
	case "error":
		return "下载失败"
	}
	return "等待中"
}
func bytesText(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	size := float64(value)
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		size /= 1024
		if size < 1024 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", size, unit)
		}
	}
	return "0 B"
}
