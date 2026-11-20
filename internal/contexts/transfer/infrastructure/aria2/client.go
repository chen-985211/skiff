package aria2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"strconv"
	"strings"
)

func (c *Client) Enqueue(ctx context.Context, s application.TransferSpec) error {
	if err := os.MkdirAll(s.Directory, 0755); err != nil {
		return err
	}
	options := map[string]string{"gid": s.ID, "dir": s.Directory, "pause": strconv.FormatBool(s.Paused)}
	if s.Version.ETag != "" {
		options["header"] = "If-Match: " + s.Version.ETag
	} else if s.Version.LastModified != "" {
		options["header"] = "If-Unmodified-Since: " + s.Version.LastModified
	}
	if s.OutputName != "" {
		options["out"] = s.OutputName
		options["continue"] = "false"
		options["split"] = "1"
		options["max-connection-per-server"] = "1"
		options["allow-overwrite"] = "false"
		options["auto-file-renaming"] = "true"
	}
	return c.call(ctx, "aria2.addUri", []any{[]string{s.URL}, options}, nil)
}
func (c *Client) Execute(ctx context.Context, id string, cmd application.Command) error {
	switch cmd {
	case application.Pause:
		return c.call(ctx, "aria2.forcePause", []any{id}, nil)
	case application.Resume:
		return c.call(ctx, "aria2.unpause", []any{id}, nil)
	case application.Remove, application.Retry, application.Restart:
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
	GID       string `json:"gid"`
	Status    string `json:"status"`
	Total     string `json:"totalLength"`
	Completed string `json:"completedLength"`
	Speed     string `json:"downloadSpeed"`
	Error     string `json:"errorMessage"`
	ErrorCode string `json:"errorCode"`
	Files     []struct {
		Path string `json:"path"`
	} `json:"files"`
}

var keys = []string{"gid", "status", "totalLength", "completedLength", "downloadSpeed", "errorMessage", "errorCode", "files"}

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
		for _, t := range tasks {
			o := application.Observation{ID: t.GID, Status: t.Status, Error: t.Error}
			if t.ErrorCode == "8" {
				o.Failure = application.ResumeUnsupported
			}
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
