package application

import (
	"context"
	"net/url"
	"path/filepath"
	"skiff/internal/contexts/transfer/domain"
	"slices"
	"strings"
)

func normalizeMirrors(text string, primary string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	seen := map[string]bool{primary: true}
	var sources []string
	for raw := range strings.Lines(text) {
		line := strings.TrimSpace(raw)
		if line == "" || seen[line] {
			continue
		}
		u, e := url.Parse(line)
		if e != nil || u.Hostname() == "" || u.User != nil || u.Scheme != "http" && u.Scheme != "https" || strings.ContainsAny(line, " \t") {
			return "", ErrInvalidMirrors
		}
		seen[line] = true
		sources = append(sources, line)
	}
	return strings.Join(sources, "\n"), nil
}
func sameOrigin(a, b string) bool {
	x, ex := url.Parse(a)
	y, ey := url.Parse(b)
	return ex == nil && ey == nil && x.Scheme == y.Scheme && strings.EqualFold(x.Host, y.Host)
}
func canFallBackToMirror(failure string) bool {
	switch failure {
	case domain.SourceUnavailable, ResumeUnsupported, domain.SourceChanged,
		string(ErrTooManyRedirects), string(ErrDownloadAuthorization), string(ErrDownloadChecksum):
		return true
	default:
		return false
	}
}

func (s *Service) tryMirror(ctx context.Context, r domain.Record) error {
	next, rest, _ := strings.Cut(r.Mirrors, "\n")
	spec := transferSpec(r, false)
	spec.URL = next
	if !sameOrigin(r.URL, next) {
		spec.Headers = ""
	}
	info, _ := s.engine.InspectRequest(ctx, spec, false)
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.tasks, func(t domain.Task) bool { return t.Record() == r })
	if i < 0 {
		return nil
	}
	task := &s.tasks[i]
	previous := *task
	if r.Digest != "" && info.Digest != "" && r.Digest != info.Digest {
		task.SkipMirror(rest)
		if err := s.persist(ctx); err != nil {
			*task = previous
			return err
		}
		return nil
	}
	partial, err := s.engine.HasPartial(ctx, r)
	if err != nil {
		return err
	}
	sameContent := r.Digest != "" && r.Digest == info.Digest || r.Version.Resource != "" && r.Version.Resource == info.Resource && r.Version.ETag != "" && r.Version.ETag == info.ETag
	if err := s.engine.Execute(ctx, r.ID, Retry); err != nil {
		return err
	}
	if (partial || r.Completed > 0) && !sameContent {
		name := filepath.Base(r.Name)
		ext := filepath.Ext(name)
		task.RestartAs(strings.TrimSuffix(name, ext) + ".mirror-" + s.nextID() + ext)
	} else if r.OutputName == "" {
		task.SetOutputName(filepath.Base(r.Name))
	}
	task.UseMirror(next, rest, info.SourceVersion)
	if err := s.persist(ctx); err != nil {
		*task = previous
		return err
	}
	if err := s.enqueue(ctx, transferSpec(task.Record(), false)); err != nil {
		task.MarkFailure(CodeOf(err), err.Error())
		return s.persist(ctx)
	}
	return nil
}
