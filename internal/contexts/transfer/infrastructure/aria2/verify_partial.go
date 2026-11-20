package aria2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
)

// CheckPartial is the fallback for a task created without a trustworthy HTTP
// validator. Read only blocks aria2 can reuse and compare every byte; never
// send local file contents to the origin or edit the user's checkpoint here.
func (c *Client) CheckPartial(ctx context.Context, r domain.Record) (application.PartialCheck, error) {
	blocked := application.PartialCheck{Failure: domain.CheckpointUnverified}
	path := r.Path
	if r.Status == domain.Paused || path == "" {
		status, err := c.stoppedFile(ctx, r.ID)
		if err != nil {
			return application.PartialCheck{}, err
		}
		if path == "" {
			path = status
		}
	}
	if path == "" {
		name := r.OutputName
		if name == "" {
			name = r.Name
		}
		path = filepath.Join(r.Directory, filepath.Base(name))
	}
	total, spans, err := checkpointSpans(path + ".aria2")
	if errors.Is(err, errCheckpoint) || errors.Is(err, os.ErrNotExist) {
		return blocked, nil
	}
	if err != nil {
		return application.PartialCheck{}, err
	}
	if r.Total > 0 && r.Total != total {
		return blocked, nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return blocked, nil
	}
	if err != nil {
		return application.PartialCheck{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return application.PartialCheck{}, err
	}
	if !info.Mode().IsRegular() || len(spans) > 0 && spans[len(spans)-1].end > info.Size() {
		return blocked, nil
	}
	// A checkpoint saved before the first full block can restart at zero.
	if len(spans) == 0 && r.Completed > 0 {
		return blocked, nil
	}
	checked := application.PartialCheck{Version: r.Version}
	local, remote := make([]byte, 64<<10), make([]byte, 64<<10)
	for _, span := range spans {
		checked, err = verifySpan(ctx, r.URL, file, span, total, checked.Version, local, remote)
		if err != nil || checked.Failure != "" {
			return checked, err
		}
	}
	return checked, nil
}

// forcePause is asynchronous. Wait until the engine has flushed its final
// checkpoint before comparing any ranges, including on a quick double click.
func (c *Client) stoppedFile(ctx context.Context, id string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		var state ariaTask
		err := c.call(ctx, "aria2.tellStatus", []any{id, []string{"status", "files"}}, &state)
		if missingGID(err, id) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if state.Status == "paused" || state.Status == "error" || state.Status == "removed" {
			if len(state.Files) > 0 {
				return state.Files[0].Path, nil
			}
			return "", nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("等待任务暂停：%w", ctx.Err())
		case <-ticker.C:
		}
	}
}

type resumeBody struct {
	io.Reader
	idle *time.Timer
}

func (b resumeBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if n > 0 {
		b.idle.Reset(30 * time.Second)
	}
	return n, err
}

func verifySpan(ctx context.Context, source string, file *os.File, span byteSpan, total int64, version domain.SourceVersion, local, remote []byte) (application.PartialCheck, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	idle := time.AfterFunc(30*time.Second, cancel)
	defer idle.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return application.PartialCheck{}, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	interval := fmt.Sprintf("%d-%d", span.start, span.end-1)
	req.Header.Set("Range", "bytes="+interval)
	if version.ETag != "" {
		req.Header.Set("If-Match", version.ETag)
	} else if version.LastModified != "" {
		req.Header.Set("If-Unmodified-Since", version.LastModified)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *url.Error
		if errors.As(err, &netErr) {
			err = netErr.Err
		}
		return application.PartialCheck{}, fmt.Errorf("暂时无法校验下载源：%w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPreconditionFailed, http.StatusRequestedRangeNotSatisfiable:
		return application.PartialCheck{Failure: domain.SourceChanged}, nil
	case http.StatusOK:
		return application.PartialCheck{Failure: domain.ResumeUnsupported}, nil
	case http.StatusPartialContent:
	default:
		return application.PartialCheck{}, fmt.Errorf("下载源暂时无法校验（HTTP %d），可以重试", resp.StatusCode)
	}
	prefix, length, ok := strings.Cut(resp.Header.Get("Content-Range"), "/")
	if !ok || prefix != "bytes "+interval || resp.Uncompressed || resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "identity" {
		return application.PartialCheck{Failure: domain.ResumeUnsupported}, nil
	}
	size, err := strconv.ParseInt(length, 10, 64)
	if err != nil || size != total {
		return application.PartialCheck{Failure: domain.SourceChanged}, nil
	}
	current := responseVersion(resp)
	if version.ETag != "" && version.ETag != current.ETag || version.LastModified != "" && version.LastModified != current.LastModified {
		return application.PartialCheck{Failure: domain.SourceChanged}, nil
	}
	body := resumeBody{resp.Body, idle}
	reader := io.NewSectionReader(file, span.start, span.end-span.start)
	for left := span.end - span.start; left > 0; {
		n := min(int64(len(local)), left)
		if _, err := io.ReadFull(reader, local[:n]); err != nil {
			return application.PartialCheck{}, err
		}
		if _, err := io.ReadFull(body, remote[:n]); err != nil {
			return application.PartialCheck{}, fmt.Errorf("下载源校验中断，可以重试：%w", err)
		}
		if !bytes.Equal(local[:n], remote[:n]) {
			return application.PartialCheck{Failure: domain.SourceChanged}, nil
		}
		left -= n
	}
	return application.PartialCheck{Version: current}, nil
}
