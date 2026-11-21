package aria2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
	"strconv"
	"strings"
)

func (c *Client) Enqueue(ctx context.Context, s application.TransferSpec) error {
	if err := os.MkdirAll(s.Directory, 0755); err != nil {
		return err
	}
	options := map[string]string{"gid": s.ID, "dir": s.Directory, "pause": strconv.FormatBool(s.Paused)}
	options["min-split-size"] = strconv.FormatInt(s.Disk.SplitBytes(), 10)
	if s.Disk.Allocation != "" {
		options["file-allocation"] = s.Disk.Allocation
	}
	if strings.HasPrefix(s.URL, "ftp://") {
		options["ftp-pasv"] = "true"
		if s.FTPUser != "" {
			options["ftp-user"] = s.FTPUser
			options["ftp-passwd"] = s.FTPPassword
		}
	}
	if s.ConnectionLimit > 0 {
		options["split"] = strconv.Itoa(s.ConnectionLimit)
		options["max-connection-per-server"] = strconv.Itoa(s.ConnectionLimit)
	}
	if header := conditionalHeader(s.Version); header != "" {
		options["header"] = header
	}
	if s.OutputName != "" {
		options["out"] = s.OutputName
		options["continue"] = "false"
		if s.ConnectionLimit == 0 {
			options["split"] = "1"
			options["max-connection-per-server"] = "1"
		}
		options["allow-overwrite"] = "false"
		options["auto-file-renaming"] = "true"
	}
	source := s.URL
	if s.Headers != "" {
		var err error
		source, err = c.routes.register(s)
		if err != nil {
			return err
		}
		options["no-proxy"] = "127.0.0.1"
		options["all-proxy"] = ""
	}
	err := c.call(ctx, "aria2.addUri", []any{[]string{source}, options}, nil)
	if err != nil {
		c.routes.remove(s.ID)
	}
	return err
}
func (c *Client) Execute(ctx context.Context, id string, cmd application.Command) error {
	switch cmd {
	case application.Prioritize:
		return c.call(ctx, "aria2.changePosition", []any{id, 0, "POS_SET"}, nil)
	case application.Pause:
		return c.call(ctx, "aria2.forcePause", []any{id}, nil)
	case application.Resume:
		return c.call(ctx, "aria2.unpause", []any{id}, nil)
	case application.Remove, application.Retry, application.Restart:
		c.routes.remove(id)
		var status struct {
			Status string `json:"status"`
		}
		if err := c.call(ctx, "aria2.tellStatus", []any{id, []string{"status"}}, &status); err != nil {
			if missingGID(err, id) {
				return nil
			}
			return err
		}
		if status.Status == "active" || status.Status == "waiting" || status.Status == "paused" {
			if err := c.call(ctx, "aria2.forceRemove", []any{id}, nil); err != nil {
				return err
			}
		}
		err := c.call(ctx, "aria2.removeDownloadResult", []any{id}, nil)
		if missingGID(err, id) {
			return nil
		}
		return err
	}
	return errors.New("未知下载操作")
}

type ariaTask struct {
	Connections string `json:"connections"`
	GID         string `json:"gid"`
	Status      string `json:"status"`
	Total       string `json:"totalLength"`
	Completed   string `json:"completedLength"`
	Speed       string `json:"downloadSpeed"`
	Error       string `json:"errorMessage"`
	ErrorCode   string `json:"errorCode"`
	Files       []struct {
		Path string `json:"path"`
	} `json:"files"`
}

var keys = []string{"gid", "status", "connections", "totalLength", "completedLength", "downloadSpeed", "errorMessage", "errorCode", "files"}

func (c *Client) Observe(ctx context.Context) ([]application.Observation, error) {
	var result []application.Observation
	for _, req := range []struct {
		method string
		args   []any
	}{
		{"aria2.tellActive", []any{keys}},
		{"aria2.tellWaiting", []any{0, 10000, keys}},
		{"aria2.tellStopped", []any{0, 10000, keys}},
	} {
		var tasks []ariaTask
		if err := c.call(ctx, req.method, req.args, &tasks); err != nil {
			return nil, err
		}
		firstWaiting := true
		for _, t := range tasks {
			if t.Status == "complete" || t.Status == "error" || t.Status == "removed" {
				c.routes.remove(t.GID)
			}
			o := application.Observation{ID: t.GID, Status: t.Status, Error: t.Error, HTTPRejected: t.ErrorCode == "22"}
			if req.method == "aria2.tellWaiting" && t.Status == "waiting" && firstWaiting {
				o.NextInQueue = true
				firstWaiting = false
			}
			if t.ErrorCode == "21" && strings.HasSuffix(strings.TrimSpace(t.Error), "status=530") {
				o.Failure = domain.FTPLoginRequired
			}
			if t.ErrorCode == "8" {
				o.Failure = application.ResumeUnsupported
			}
			o.Connections, _ = strconv.Atoi(t.Connections)
			o.Total, _ = strconv.ParseInt(t.Total, 10, 64)
			o.Completed, _ = strconv.ParseInt(t.Completed, 10, 64)
			o.Speed, _ = strconv.ParseInt(t.Speed, 10, 64)
			if len(t.Files) > 0 && t.Files[0].Path != "" {
				o.Path = t.Files[0].Path
				o.Name = filepath.Base(o.Path)
			}
			result = append(result, o)
		}
	}
	return result, nil
}

func missingGID(err error, id string) bool {
	var rpcErr *RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == 1 && strings.Contains(rpcErr.Message, id) && strings.Contains(rpcErr.Message, "not found")
}

func (c *Client) SetConcurrency(ctx context.Context, limit int) error {
	return c.call(ctx, "aria2.changeGlobalOption", []any{map[string]string{"max-concurrent-downloads": strconv.Itoa(limit)}}, nil)
}

func conditionalHeader(v domain.SourceVersion) string {
	if v.ETag != "" {
		return "If-Match: " + v.ETag
	}
	if v.LastModified != "" {
		return "If-Unmodified-Since: " + v.LastModified
	}
	return ""
}
