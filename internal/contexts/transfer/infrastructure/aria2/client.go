package aria2

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
	"strconv"
	"strings"
)

func (c *Client) Enqueue(ctx context.Context, s application.TransferSpec) error {
	if s.TorrentHash != "" {
		return c.enqueueTorrent(ctx, s)
	}
	if err := os.MkdirAll(s.Directory, 0755); err != nil {
		return downloadDirectoryError(err)
	}
	options := map[string]string{"gid": s.ID, "dir": s.Directory, "pause": strconv.FormatBool(s.Paused)}
	options["max-download-limit"] = strconv.FormatInt(s.DownloadLimit, 10)
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
	if s.Digest != "" {
		options["checksum"] = "sha-256=" + s.Digest
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
	if err := engineNetwork(s, options); err != nil {
		return err
	}
	if needsLocalRoute(s) {
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

func downloadDirectoryError(err error) error {
	if isVolumeFull(err) {
		return application.Wrap(application.ErrInsufficientDiskSpace, err)
	}
	return application.Wrap(application.ErrDirectoryUnavailable, err)
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
		var status struct {
			Status string `json:"status"`
		}
		if err := c.call(ctx, "aria2.tellStatus", []any{id, []string{"status"}}, &status); err != nil {
			if missingGID(err, id) {
				c.routes.remove(id)
				return nil
			}
			return err
		}
		if status.Status == "active" || status.Status == "waiting" || status.Status == "paused" {
			if err := c.call(ctx, "aria2.forceRemove", []any{id}, nil); err != nil {
				return err
			}
			c.routes.remove(id)
		}
		if err := c.call(ctx, "aria2.removeDownloadResult", []any{id}, nil); err != nil && !missingGID(err, id) {
			return err
		}
		c.routes.remove(id)
		return nil
	}
	return application.ErrCommandUnavailable
}

type ariaTask struct {
	Seeder      string `json:"seeder"`
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
		URIs []struct {
			URI string `json:"uri"`
		} `json:"uris"`
	} `json:"files"`
}

var keys = []string{"seeder", "gid", "status", "connections", "totalLength", "completedLength", "downloadSpeed", "errorMessage", "errorCode", "files"}

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
			o := application.Observation{Seeding: t.Seeder == "true", ID: t.GID, Status: t.Status, Error: t.Error, HTTPRejected: t.ErrorCode == "22"}
			if req.method == "aria2.tellWaiting" && t.Status == "waiting" && firstWaiting {
				o.NextInQueue = true
				firstWaiting = false
			}
			switch t.ErrorCode {
			case "1", "2", "5", "6", "7", "19", "21", "29":
				o.Failure = domain.SourceUnavailable
			case "3", "4":
				o.Failure = string(application.ErrDownloadNotFound)
			case "22":
				o.Failure = domain.SourceUnavailable
				// aria2 reports the HTTP status in its generic protocol error.
				if strings.HasSuffix(strings.TrimSpace(t.Error), " status=410") {
					o.Failure = string(application.ErrDownloadNotFound)
				}
			case "9":
				o.Failure = string(application.ErrInsufficientDiskSpace)
			case "16":
				o.Failure = string(application.ErrCreateDownloadFile)
			case "18":
				o.Failure = string(application.ErrDirectoryUnavailable)
			case "23":
				o.Failure = string(application.ErrTooManyRedirects)
			case "24":
				o.Failure = string(application.ErrDownloadAuthorization)
			case "32":
				o.Failure = string(application.ErrDownloadChecksum)
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
			if len(t.Files) > 0 {
				for _, entry := range t.Files[0].URIs {
					u, e := url.Parse(entry.URI)
					if e == nil && u.Hostname() == "127.0.0.1" && strings.HasPrefix(u.Path, "/_skiff_transfer/") {
						o.LocalRoute = true
					}
				}
			}
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

func (c *Client) SetDownloadLimit(ctx context.Context, id string, rate int64) error {
	value := strconv.FormatInt(rate, 10)
	if id == "" {
		return c.call(ctx, "aria2.changeGlobalOption", []any{map[string]string{"max-overall-download-limit": value}}, nil)
	}
	return c.call(ctx, "aria2.changeOption", []any{id, map[string]string{"max-download-limit": value}}, nil)
}
