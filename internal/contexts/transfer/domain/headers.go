package domain

import (
	"strings"
)

// NormalizeHeaders accepts literal HTTP fields only. Transport and range headers
// belong to the download engine and must retain their original semantics.
func NormalizeHeaders(block string) (string, error) {
	var lines []string
	for line := range strings.Lines(block) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || name == "" || strings.IndexFunc(name, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r))
		}) >= 0 || strings.IndexFunc(value, func(r rune) bool { return r < 32 && r != '\t' || r == 127 }) >= 0 {
			return "", ErrInvalidHeaders
		}
		switch strings.ToLower(name) {
		case "host", "connection", "upgrade", "transfer-encoding", "te", "trailer", "content-length", "range", "accept-encoding", "keep-alive", "proxy-connection", "proxy-authorization", "if-match", "if-unmodified-since", "if-range":
			return "", ErrManagedHeader
		}
		lines = append(lines, name+": "+value)
	}
	return strings.Join(lines, "\n"), nil
}
func (t *Task) SetHeaders(value string) { t.record.Headers = value }
