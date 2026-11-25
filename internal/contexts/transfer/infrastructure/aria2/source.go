package aria2

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
)

func (c *Client) InspectSource(ctx context.Context, source string, ranges bool) (application.SourceInfo, error) {
	return c.InspectRequest(ctx, application.TransferSpec{URL: source}, ranges)
}
func (c *Client) InspectRequest(ctx context.Context, s application.TransferSpec, ranges bool) (application.SourceInfo, error) {
	source := s.URL
	if strings.HasPrefix(source, "ftp://") {
		return application.SourceInfo{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, source, nil)
	if err != nil {
		return application.SourceInfo{}, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	client := sourceClient(s)
	applyHeaders(req, s.Headers)
	resp, err := client.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
		}
		return application.SourceInfo{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotImplemented, http.StatusMethodNotAllowed, http.StatusForbidden:
		return inspectRange(client, req, ranges, application.SourceInfo{})
	case http.StatusNotFound, http.StatusGone, http.StatusUnauthorized:
		return application.SourceInfo{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return application.SourceInfo{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	info := responseInfo(resp)
	if ranges && resp.ContentLength >= 2*s.Disk.SplitBytes() && resp.Header.Get("Accept-Ranges") != "none" {
		return inspectRange(client, req, true, info)
	}
	return info, nil
}

func responseInfo(resp *http.Response) application.SourceInfo {
	info := application.SourceInfo{SourceVersion: responseVersion(resp), Filename: path.Base(resp.Request.URL.Path)}
	for part := range strings.SplitSeq(resp.Header.Get("Repr-Digest"), ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(key, "sha-256") {
			if b, e := base64.StdEncoding.DecodeString(strings.Trim(value, ":")); e == nil && len(b) == 32 {
				info.Digest = hex.EncodeToString(b)
			}
		}
	}
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && params["filename"] != "" {
		info.Filename = params["filename"]
	}
	if info.Filename == "." || info.Filename == "/" {
		info.Filename = ""
	}
	return info
}

func inspectRange(client *http.Client, head *http.Request, ranges bool, info application.SourceInfo) (application.SourceInfo, error) {
	probe := head.Clone(head.Context())
	probe.Method = http.MethodGet
	probe.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(probe)
	if err != nil {
		return info, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return info, nil
	}
	actual := responseInfo(resp)
	if actual.Verified() || !info.Verified() {
		info = actual
	}
	if value, ok := strings.CutPrefix(resp.Header.Get("Content-Range"), "bytes 0-0/"); ok && resp.StatusCode == http.StatusPartialContent && ranges {
		if total, err := strconv.ParseInt(value, 10, 64); err == nil && total > 0 {
			info.RangeBytes = total
		}
	}
	return info, nil
}

func responseVersion(resp *http.Response) domain.SourceVersion {
	v := domain.SourceVersion{Resource: resp.Request.URL.String()}
	tag := resp.Header.Get("ETag")
	if len(tag) >= 2 && strings.HasPrefix(tag, `"`) && strings.HasSuffix(tag, `"`) && !strings.ContainsAny(tag[1:len(tag)-1], "\r\n\"") {
		v.ETag = tag
	} else if modified, err := http.ParseTime(resp.Header.Get("Last-Modified")); err == nil {
		// HTTP dates have one-second resolution. Only use an old enough
		// Last-Modified value as a strong validator (RFC 9110 §8.8.2.2).
		if date, err := http.ParseTime(resp.Header.Get("Date")); err == nil && date.Sub(modified) >= time.Minute {
			v.LastModified = modified.UTC().Format(http.TimeFormat)
		}
	}
	return v
}

// A crash or a quick pause can leave a checkpoint before the first progress poll.
func (c *Client) HasPartial(ctx context.Context, r domain.Record) (bool, error) {
	path := r.Path
	if path == "" {
		var status ariaTask
		err := c.call(ctx, "aria2.tellStatus", []any{r.ID, []string{"files", "completedLength"}}, &status)
		if err != nil && !missingGID(err, r.ID) {
			return false, err
		}
		if status.Completed != "" && status.Completed != "0" {
			return true, nil
		}
		if len(status.Files) > 0 {
			path = status.Files[0].Path
		}
	}
	if path == "" {
		name := r.OutputName
		if name == "" {
			name = r.Name
		}
		if name == "" {
			return false, nil
		}
		path = filepath.Join(r.Directory, filepath.Base(name))
	}
	for _, candidate := range []string{path, path + ".aria2"} {
		f, err := os.Stat(candidate)
		if err == nil && f.Mode().IsRegular() && f.Size() > 0 {
			return true, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

// aria2 will open a same-name control file even with continue=false. When the
// destination contains checkpoints, pin an unused output before enqueue so a
// newly submitted task never claims the bytes of a removed or foreign task.
func (c *Client) ProtectCheckpoints(ctx context.Context, dir string, candidates []application.OutputCandidate) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, downloadDirectoryError(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := directoryWriteAccess(dir); err != nil {
		return nil, downloadDirectoryError(&os.PathError{Op: "check directory write access", Path: dir, Err: err})
	}
	hasCheckpoint := false
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".aria2") {
			hasCheckpoint = true
			break
		}
	}
	if !hasCheckpoint {
		return nil, nil
	}
	occupied := make(map[string]bool, len(entries))
	for _, entry := range entries {
		occupied[strings.ToLower(entry.Name())] = true
	}
	chosen := make(map[string]string, len(candidates))
	for _, item := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := item.Name
		if !filepath.IsLocal(name) || filepath.Base(name) != name || name == "." || strings.ContainsAny(name, "<>:\"/\\|?*\x00\r\n") {
			name = "download-" + item.ID
		}
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		for i := 1; occupied[strings.ToLower(name)] || occupied[strings.ToLower(name)+".aria2"]; i++ {
			name = fmt.Sprintf("%s.%d%s", stem, i, ext)
		}
		chosen[item.ID] = name
		occupied[strings.ToLower(name)] = true
	}
	return chosen, nil
}
