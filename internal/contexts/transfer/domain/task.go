package domain

import (
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type Status string

const (
	Waiting  Status = "waiting"
	Active   Status = "active"
	Paused   Status = "paused"
	Complete Status = "complete"
	Failed   Status = "error"
)

type Command string

const ResumeUnsupported = "resume_unsupported"
const SourceUnverified = "source_unverified"

// SourceVersion identifies the representation selected by the original HTTP request.
type SourceVersion struct {
	Resource, ETag, LastModified string
}

func (v SourceVersion) Verified() bool { return v.ETag != "" || v.LastModified != "" }

const (
	Prioritize Command = "prioritize"
	Pause      Command = "pause"
	Resume     Command = "resume"
	Retry      Command = "retry"
	Restart    Command = "restart"
	Remove     Command = "remove"
)

const (
	DefaultConnectionLimit = 4
	MaxConnectionLimit     = 16
	MinSplitBytes          = 4 << 20
)

type Record struct {
	RequestedConnections int `json:",omitempty"`
	ConnectionLimit      int `json:",omitempty"`
	Version              SourceVersion
	ID                   string
	URL                  string
	Name                 string
	Directory            string
	Path                 string
	Status               Status
	Total                int64
	Completed            int64
	Speed                int64
	Error                string
	Failure              string
	OutputName           string
}
type Task struct {
	record      Record
	connections int
	nextInQueue bool
}

type Observation struct {
	NextInQueue             bool
	Connections             int
	Status                  Status
	Name, Path, Error       string
	Failure                 string
	Total, Completed, Speed int64
}

func (t *Task) Observe(o Observation) {
	switch o.Status {
	case Waiting, Active, Paused, Failed:
		// A restored transfer can report no lengths until its file is opened.
		if o.Total == 0 && o.Completed == 0 {
			o.Total, o.Completed = t.record.Total, t.record.Completed
		}
	case Complete:
	default:
		return
	}
	t.record.Status, t.record.Error, t.record.Failure = o.Status, o.Error, o.Failure
	t.record.Total, t.record.Completed, t.record.Speed = max(0, o.Total), max(0, o.Completed), max(0, o.Speed)
	t.nextInQueue = o.Status == Waiting && o.NextInQueue
	t.connections = max(0, o.Connections)
	if o.Status != Active {
		t.connections = 0
		t.record.Speed = 0
	}
	if o.Name != "" && t.record.OutputName == "" {
		t.record.Name = o.Name
	}
	if o.Path != "" {
		t.record.Path = o.Path
	}
}

func (t *Task) Mark(status Status, detail string) {
	o := t.record
	t.Observe(Observation{Status: status, Total: o.Total, Completed: o.Completed, Name: o.Name, Path: o.Path, Error: detail})
}

func (t Task) NextInQueue() bool { return t.nextInQueue }
func (t Task) Connections() int  { return t.connections }

func (t *Task) PlanConnections(requested int, rangeBytes int64) {
	limit := requested
	if limit == 0 {
		limit = DefaultConnectionLimit
	}
	t.record.RequestedConnections = requested
	t.record.ConnectionLimit = int(max(1, min(int64(limit), rangeBytes/MinSplitBytes)))
}

func (t *Task) SetVersion(v SourceVersion) { t.record.Version = v }

func (t *Task) RefuseResume() {
	t.Mark(Failed, "")
	t.record.Failure = SourceUnverified
}

func (t *Task) RestartAs(outputName string) {
	t.PlanConnections(1, 0)
	t.record.OutputName = outputName
	t.record.Path, t.record.Error, t.record.Failure = "", "", ""
	t.record.Total, t.record.Completed, t.record.Speed = 0, 0, 0
	t.record.Status = Waiting
}

func (t Task) FailureDescription() string {
	if t.record.Failure == SourceUnverified {
		return "无法确认远端文件版本，当前文件不支持断点续传；可保留原文件并从头下载"
	}
	if t.record.Failure == ResumeUnsupported {
		return "服务器不支持断点续传，请选择从头下载"
	}
	return t.record.Error
}

func NewTask(id, source, directory string) Task {
	u, _ := url.Parse(source)
	name := path.Base(u.Path)
	if name == "." || name == "/" {
		name = u.Hostname()
	}
	return Task{record: Record{ID: id, URL: source, Directory: directory, Name: name, Status: Waiting}}
}

// Older versions also saved the generated disk name as the task's title.
var restartSuffix = regexp.MustCompile(`(\.restart-[0-9a-f]{16})+(\.[^.]+)?$`)

func Restore(record Record) Task {
	if record.ConnectionLimit <= 0 {
		record.ConnectionLimit = DefaultConnectionLimit
		if record.OutputName != "" {
			record.ConnectionLimit = 1
			record.RequestedConnections = 1
		}
	}
	if record.OutputName != "" && record.Name == record.OutputName {
		record.Name = restartSuffix.ReplaceAllString(record.Name, "${2}")
	}
	return Task{record: record}
}
func (t Task) Record() Record { return t.record }
func (t Task) Allows(command Command) bool {
	switch command {
	case Prioritize:
		return t.record.Status == Waiting && !t.nextInQueue
	case Remove:
		return true
	case Pause:
		return t.record.Status == Active || t.record.Status == Waiting
	case Resume:
		return t.record.Status == Paused
	case Retry:
		return t.record.Status == Failed && t.record.Failure != ResumeUnsupported && t.record.Failure != SourceUnverified
	case Restart:
		return t.record.Status == Failed
	}
	return false
}
func (t Task) Progress() float64 {
	if t.record.Status == Complete {
		return 1
	}
	if t.record.Total <= 0 {
		return 0
	}
	return max(0, min(1, float64(t.record.Completed)/float64(t.record.Total)))
}

type Batch struct {
	Sources, Remaining            []string
	Duplicates, Invalid, Deferred int
}

func ParseBatch(text string) (Batch, error) {
	var batch Batch
	seen := make(map[string]bool)
	for raw := range strings.Lines(text) {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		u, err := url.Parse(line)
		switch {
		case err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || strings.ContainsAny(line, " \t\r\n"):
			batch.Invalid++
			batch.Remaining = append(batch.Remaining, line)
		case seen[line]:
			batch.Duplicates++
		case len(batch.Sources) >= 100:
			seen[line] = true
			batch.Deferred++
			batch.Remaining = append(batch.Remaining, line)
		default:
			seen[line] = true
			batch.Sources = append(batch.Sources, line)
		}
	}
	if len(batch.Sources) == 0 && batch.Invalid == 0 {
		return batch, errors.New("请先输入下载链接")
	}
	return batch, nil
}
