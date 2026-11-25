package application

import (
	"context"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"skiff/internal/contexts/transfer/domain"
)

const MaxConnectionLimit = domain.MaxConnectionLimit

func (s *Service) Add(ctx context.Context, text, directory string) (int, error) {
	result, err := s.AddBatch(ctx, text, directory)
	if err == nil && len(result.Remaining) > 0 {
		err = ErrBatchRemaining
	}
	return result.Added, err
}

type BatchResult struct {
	Added, Duplicates, Invalid, Deferred int
	Remaining                            []string
}

func (s *Service) AddBatch(ctx context.Context, text, directory string) (BatchResult, error) {
	return s.AddBatchWithConnections(ctx, text, directory, 0)
}

func (s *Service) AddBatchWithConnections(ctx context.Context, text, directory string, limit int) (BatchResult, error) {
	return s.AddWithOptions(ctx, text, directory, AddOptions{ConnectionLimit: limit})
}

type AddOptions struct {
	Network         NetworkSettings `json:",omitzero"`
	Mirrors         string
	Headers         string
	OutputName      string
	ConnectionLimit int
}

func (s *Service) AddWithOptions(ctx context.Context, text, directory string, opts AddOptions) (BatchResult, error) {
	if request, ok, err := ImportCurl(text); ok {
		if err != nil {
			return BatchResult{}, err
		}
		text = request.URL
		opts.Headers = strings.TrimSpace(request.Headers + "\n" + opts.Headers)
	}
	limit := opts.ConnectionLimit
	if limit < 0 || limit > MaxConnectionLimit {
		return BatchResult{}, ErrConnectionLimit
	}
	batch, err := domain.ParseBatch(text)
	if err != nil {
		return BatchResult{}, err
	}
	name := strings.TrimSpace(opts.OutputName)
	if name != "" && (len(batch.Sources) != 1 || len(batch.Remaining) > 0 || !filepath.IsLocal(name) || filepath.Base(name) != name || name == "." || strings.ContainsAny(name, `<>:"/\|?*`+"\x00\r\n")) {
		return BatchResult{}, ErrInvalidOutputName
	}
	directory = filepath.Clean(strings.TrimSpace(directory))
	if !filepath.IsAbs(directory) {
		return BatchResult{}, ErrInvalidDirectory
	}
	result := BatchResult{Duplicates: batch.Duplicates, Invalid: batch.Invalid, Deferred: batch.Deferred, Remaining: batch.Remaining}
	if len(batch.Sources) == 0 {
		return result, nil
	}
	if opts.Mirrors != "" && (len(batch.Sources) != 1 || len(batch.Remaining) > 0 || strings.HasPrefix(batch.Sources[0], "ftp://")) {
		return BatchResult{}, ErrSingleMirrorTask
	}
	mirrors, err := normalizeMirrors(opts.Mirrors, batch.Sources[0])
	if err != nil {
		return BatchResult{}, err
	}
	headers, err := domain.NormalizeHeaders(opts.Headers)
	if err != nil {
		return BatchResult{}, err
	}
	s.mu.Lock()
	disk := s.preferences.Disk
	network := opts.Network
	if network.Mode == "" {
		network = s.preferences.Network
	} else if network.DNS == "" {
		network.DNS = s.preferences.Network.DNS
	}
	s.mu.Unlock()
	if err := network.Validate(); err != nil {
		return BatchResult{}, err
	}
	requests := make([]TransferSpec, len(batch.Sources))
	origin := ""
	for i, source := range batch.Sources {
		if headers != "" {
			u, _ := url.Parse(source)
			current := strings.ToLower(u.Scheme + "://" + u.Host)
			if u.Scheme == "ftp" || origin != "" && current != origin {
				return BatchResult{}, ErrMixedHeaderOrigins
			}
			origin = current
		}
		requests[i] = TransferSpec{Network: network, URL: source, Headers: headers, Disk: disk}
	}
	sources := s.inspectBatch(ctx, requests, limit != 1)
	if err := ctx.Err(); err != nil {
		return BatchResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := make(map[string]bool)
	for _, task := range s.tasks {
		r := task.Record()
		if r.Status != domain.Complete && r.Status != domain.Failed && filepath.Clean(r.Directory) == directory {
			pending[r.URL] = true
		}
	}
	oldTasks, oldDir := slices.Clone(s.tasks), s.directory
	var candidates []OutputCandidate
	for i, source := range batch.Sources {
		task := domain.NewTask("", source, directory)
		if pending[task.Record().URL] {
			result.Duplicates++
			continue
		}
		task = domain.NewTask(s.nextID(), source, directory)
		task.SetMirrors(mirrors, sources[i].Digest)
		task.SetNetwork(network)
		task.SetDisk(disk)
		task.SetHeaders(headers)
		pending[task.Record().URL] = true
		if name != "" {
			task.SetOutputName(name)
		}
		name := task.Record().OutputName
		if name == "" {
			name = sources[i].Filename
		}
		if name == "" {
			name = task.Record().Name
		}
		candidates = append(candidates, OutputCandidate{ID: task.Record().ID, Name: name})
		task.SetVersion(sources[i].SourceVersion)
		task.PlanConnections(limit, sources[i].RangeBytes)
		s.tasks = append(s.tasks, task)
	}
	result.Added = len(s.tasks) - len(oldTasks)
	if result.Added == 0 {
		return result, nil
	}
	protected, err := s.engine.ProtectCheckpoints(ctx, directory, candidates)
	if err != nil {
		s.tasks = oldTasks
		return BatchResult{}, err
	}
	for i := len(oldTasks); i < len(s.tasks); i++ {
		if name, ok := protected[s.tasks[i].Record().ID]; ok {
			s.tasks[i].SetOutputName(name)
		}
	}
	recent := s.preferences.RecentDirectories
	s.preferences.RecentDirectories = [5]string{directory}
	at := 1
	for _, path := range recent {
		if path != "" && path != directory && at < len(recent) {
			s.preferences.RecentDirectories[at] = path
			at++
		}
	}
	s.directory = directory
	// The persistent intent precedes the external effect, so a crash can be reconciled.
	if err := s.persist(ctx); err != nil {
		s.preferences.RecentDirectories = recent
		s.tasks, s.directory = oldTasks, oldDir
		return BatchResult{}, err
	}
	var failures []error
	for i := len(oldTasks); i < len(s.tasks); i++ {
		task := &s.tasks[i]
		if err := s.enqueue(ctx, transferSpec(task.Record(), false)); err != nil {
			task.MarkFailure(CodeOf(err), err.Error())
			failures = append(failures, err)
		}
	}
	if err := s.persist(ctx); err != nil {
		failures = append(failures, err)
	}
	return result, joinOperationFailures(failures)
}

// Bound metadata latency for an entire paste. Sources that do not answer HEAD
// can still download; their partial files require an explicit fresh start.
func (s *Service) inspectBatch(ctx context.Context, sources []TransferSpec, ranges bool) []SourceInfo {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	versions := make([]SourceInfo, len(sources))
	jobs := make(chan int, len(sources))
	for i := range sources {
		jobs <- i
	}
	close(jobs)
	var workers sync.WaitGroup
	for range min(8, len(sources)) {
		workers.Go(func() {
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				versions[i], _ = s.engine.InspectRequest(ctx, sources[i], ranges)
			}
		})
	}
	workers.Wait()
	return versions
}

func ImportHeaders(text string) (string, error) { return domain.NormalizeHeaders(text) }
