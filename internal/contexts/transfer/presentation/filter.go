package presentation

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"skiff/internal/contexts/transfer/application"
)

func (v *View) filtered() []application.TaskDTO {
	var tasks []application.TaskDTO
	q := strings.ToLower(strings.TrimSpace(v.Query))
	for i := len(v.Snapshot.Tasks) - 1; i >= 0; i-- {
		t := v.Snapshot.Tasks[i]
		_, wasVisible := v.visibleTransfers[t.ID]
		leavingActive := v.Filter == "active" && t.Status == "complete" && wasVisible
		if (matches(t, v.Filter) || leavingActive) && (q == "" || strings.Contains(strings.ToLower(t.Name+" "+t.URL), q)) {
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
func statusText(c *ui.Context, status string) string {
	switch status {
	case "active":
		return tr(c, "task.downloading")
	case "waiting":
		return tr(c, "task.waiting")
	case "paused":
		return tr(c, "task.paused")
	case "complete":
		return tr(c, "task.complete")
	case "error":
		return tr(c, "task.failed")
	}
	return tr(c, "task.waiting")
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
