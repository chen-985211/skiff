package aria2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/domain"
)

func (c *Client) InspectSource(ctx context.Context, source string, ranges bool) (application.SourceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, source, nil)
	if err != nil {
		return application.SourceInfo{}, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
		}
		return application.SourceInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		return application.SourceInfo{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return application.SourceInfo{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
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
	info := application.SourceInfo{SourceVersion: v}
	if ranges && resp.ContentLength >= 2*domain.MinSplitBytes && resp.Header.Get("Accept-Ranges") != "none" {
		probe := req.Clone(ctx)
		probe.Method = http.MethodGet
		probe.Header.Set("Range", "bytes=0-0")
		ranged, err := http.DefaultClient.Do(probe)
		if err == nil {
			defer ranged.Body.Close()
			if ranged.StatusCode == http.StatusPartialContent && ranged.Header.Get("Content-Range") == fmt.Sprintf("bytes 0-0/%d", resp.ContentLength) {
				info.RangeBytes = resp.ContentLength
			}
		}
	}
	return info, nil
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
