package application

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"skiff/internal/contexts/transfer/domain"
	"slices"
	"strings"
	"sync"
	"time"
)

type Command = domain.Command

const (
	ResumeUnsupported = domain.ResumeUnsupported
	Pause             = domain.Pause
	Resume            = domain.Resume
	Retry             = domain.Retry
	Restart           = domain.Restart
	Remove            = domain.Remove
)

type TransferSpec struct {
	Version            domain.SourceVersion
	ID, URL, Directory string
	OutputName         string
	Paused             bool
}
type Observation struct {
	ID, Status, Name, Path, Error string
	Failure                       string
	Total, Completed, Speed       int64
}
type Engine interface {
	HasPartial(context.Context, domain.Record) (bool, error)
	InspectSource(context.Context, string) (domain.SourceVersion, error)
	Enqueue(context.Context, TransferSpec) error
	Execute(context.Context, string, Command) error
	Observe(context.Context) ([]Observation, error)
}
type ThemeMode string

const (
	ThemeSystem ThemeMode = ""
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

func (mode ThemeMode) valid() bool {
	return mode == ThemeSystem || mode == ThemeLight || mode == ThemeDark
}

type Preferences struct {
	Theme            ThemeMode `json:",omitempty"`
	DefaultDirectory string
	StaticBackground bool
}

type State struct {
	Preferences Preferences
	Version     int
	Directory   string
	Tasks       []domain.Record
}
type Repository interface {
	Load(context.Context) (State, error)
	Save(context.Context, State) error
}
type TaskDTO struct {
	ID, URL, Name, Directory, Path, Status, Error string
	Total, Completed, Speed                       int64
	Progress                                      float64
	CanPause, CanResume, CanRetry                 bool
	CanRestart                                    bool
}
type Snapshot struct {
	Preferences Preferences
	Tasks       []TaskDTO
	Directory   string
	Speed       int64
	Active      int
}

// Service is the single serialized entry point for download intent and progress.
// Call it from workers, never from the UI thread.
type Service struct {
	preferences Preferences
	mu          sync.Mutex
	engine      Engine
	repo        Repository
	directory   string
	nextID      func() string
	tasks       []domain.Task
	dirty       bool
	lastSave    time.Time
}

func New(engine Engine, repo Repository, directory string, nextID func() string) *Service {
	return &Service{engine: engine, repo: repo, directory: directory, nextID: nextID}
}
func (s *Service) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	s.preferences = state.Preferences
	if !s.preferences.Theme.valid() {
		s.preferences.Theme = ThemeSystem
	}
	if state.Directory != "" {
		s.directory = state.Directory
	}
	s.tasks = nil
	for _, record := range state.Tasks {
		s.tasks = append(s.tasks, domain.Restore(record))
	}
	observations, err := s.engine.Observe(ctx)
	if err != nil {
		return err
	}
	found := map[string]Observation{}
	for _, o := range observations {
		found[o.ID] = o
	}
	for i := range s.tasks {
		task := &s.tasks[i]
		r := task.Record()
		if r.Status == domain.Complete || r.Status == domain.Failed {
			// A force-saved aria2 session can resurrect terminal history as
			// paused transfers. Remove only that runtime copy, never its files.
			if _, exists := found[r.ID]; exists {
				if err := s.engine.Execute(ctx, r.ID, Remove); err != nil {
					return err
				}
			}
			continue
		}
		if o, ok := found[r.ID]; ok {
			apply(task, o)
			if task.Allows(domain.Pause) {
				if err := s.engine.Execute(ctx, r.ID, Pause); err != nil {
					return err
				}
				task.Mark(domain.Paused, "")
			}
		} else {
			err := s.engine.Enqueue(ctx, TransferSpec{Version: r.Version, ID: r.ID, URL: r.URL, Directory: r.Directory, OutputName: r.OutputName, Paused: true})
			if err != nil {
				task.Mark(domain.Failed, "恢复失败："+err.Error())
			} else {
				task.Mark(domain.Paused, "")
			}
		}
	}
	return s.persist(ctx)
}
func (s *Service) Add(ctx context.Context, text, directory string) (int, error) {
	result, err := s.AddBatch(ctx, text, directory)
	if err == nil && len(result.Remaining) > 0 {
		err = fmt.Errorf("还有 %d 行待处理", len(result.Remaining))
	}
	return result.Added, err
}

type BatchResult struct {
	Added, Duplicates, Invalid, Deferred int
	Remaining                            []string
}

func (s *Service) AddBatch(ctx context.Context, text, directory string) (BatchResult, error) {
	batch, err := domain.ParseBatch(text)
	if err != nil {
		return BatchResult{}, err
	}
	directory = filepath.Clean(strings.TrimSpace(directory))
	if !filepath.IsAbs(directory) {
		return BatchResult{}, errors.New("请选择绝对路径作为保存目录")
	}
	result := BatchResult{Duplicates: batch.Duplicates, Invalid: batch.Invalid, Deferred: batch.Deferred, Remaining: batch.Remaining}
	if len(batch.Sources) == 0 {
		return result, nil
	}
	versions := s.inspectBatch(ctx, batch.Sources)
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
	for i, source := range batch.Sources {
		if pending[source] {
			result.Duplicates++
			continue
		}
		task := domain.NewTask(s.nextID(), source, directory)
		task.SetVersion(versions[i])
		s.tasks = append(s.tasks, task)
	}
	result.Added = len(s.tasks) - len(oldTasks)
	if result.Added == 0 {
		return result, nil
	}
	s.directory = directory
	// The persistent intent precedes the external effect, so a crash can be reconciled.
	if err := s.persist(ctx); err != nil {
		s.tasks, s.directory = oldTasks, oldDir
		return BatchResult{}, err
	}
	var failures []error
	for i := len(oldTasks); i < len(s.tasks); i++ {
		task := &s.tasks[i]
		r := task.Record()
		if err := s.engine.Enqueue(ctx, TransferSpec{Version: r.Version, ID: r.ID, URL: r.URL, Directory: r.Directory}); err != nil {
			task.Mark(domain.Failed, err.Error())
			failures = append(failures, err)
		}
	}
	if err := s.persist(ctx); err != nil {
		failures = append(failures, err)
	}
	return result, errors.Join(failures...)
}

// Bound metadata latency for an entire paste. Sources that do not answer HEAD
// can still download; their partial files require an explicit fresh start.
func (s *Service) inspectBatch(ctx context.Context, sources []string) []domain.SourceVersion {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	versions := make([]domain.SourceVersion, len(sources))
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
				versions[i], _ = s.engine.InspectSource(ctx, sources[i])
			}
		})
	}
	workers.Wait()
	return versions
}

func (s *Service) Execute(ctx context.Context, id string, command Command) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := slices.IndexFunc(s.tasks, func(t domain.Task) bool { return t.Record().ID == id })
	if index < 0 {
		return errors.New("任务不存在")
	}
	return s.execute(ctx, index, command)
}

// SetPaused applies a queue action to the given task IDs. Refreshes and other
// commands are serialized with the batch. Tasks that changed state before
// the action arrived are rechecked against the aggregate's current rules.
func (s *Service) SetPaused(ctx context.Context, ids []string, paused bool) error {
	command := Resume
	if paused {
		command = Pause
	}
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	var failures []error
	for i := range s.tasks {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		record := s.tasks[i].Record()
		if _, ok := selected[record.ID]; !ok || !s.tasks[i].Allows(command) {
			continue
		}
		if err := s.execute(ctx, i, command); err != nil {
			failures = append(failures, fmt.Errorf("%s：%w", record.Name, err))
		}
	}
	return errors.Join(failures...)
}

// execute requires s.mu and persists each accepted change before returning.
func (s *Service) execute(ctx context.Context, index int, command Command) error {
	task := &s.tasks[index]
	if !task.Allows(command) {
		return fmt.Errorf("当前任务不能执行 %s", command)
	}
	r := task.Record()
	id := r.ID
	if command == Resume || command == Retry {
		partial := r.Completed > 0
		if !partial {
			var err error
			partial, err = s.engine.HasPartial(ctx, r)
			if err != nil {
				return err
			}
		}
		if partial {
			var current domain.SourceVersion
			if r.Version.Verified() {
				var err error
				current, err = s.engine.InspectSource(ctx, r.URL)
				if err != nil {
					return fmt.Errorf("暂时无法检查下载源，请稍后重试：%w", err)
				}
			}
			if !r.Version.Verified() || current != r.Version {
				task.RefuseResume()
				return s.persist(ctx)
			}
		} else if command == Retry {
			version, _ := s.engine.InspectSource(ctx, r.URL)
			task.SetVersion(version)
			if err := s.persist(ctx); err != nil {
				task.SetVersion(r.Version)
				return err
			}
			r = task.Record()
		}
	}
	if command == Restart {
		previous := *task
		name := filepath.Base(r.Name)
		ext := filepath.Ext(name)
		outputName := strings.TrimSuffix(name, ext) + ".restart-" + s.nextID() + ext
		version, _ := s.engine.InspectSource(ctx, r.URL)
		task.RestartAs(outputName)
		task.SetVersion(version)
		// Commit the new target before any external effect. The previous partial
		// file and its control file are never removed or overwritten.
		if err := s.persist(ctx); err != nil {
			*task = previous
			return err
		}
		if err := s.engine.Execute(ctx, id, Restart); err != nil {
			task.Mark(domain.Failed, err.Error())
			_ = s.persist(ctx)
			return err
		}
		if err := s.engine.Enqueue(ctx, TransferSpec{Version: version, ID: id, URL: r.URL, Directory: r.Directory, OutputName: outputName}); err != nil {
			task.Mark(domain.Failed, err.Error())
			_ = s.persist(ctx)
			return err
		}
	} else if command == Retry {
		if err := s.engine.Execute(ctx, id, Retry); err != nil {
			return err
		}
		if err := s.engine.Enqueue(ctx, TransferSpec{Version: r.Version, ID: id, URL: r.URL, Directory: r.Directory, OutputName: r.OutputName}); err != nil {
			task.Mark(domain.Failed, err.Error())
			_ = s.persist(ctx)
			return err
		}
		task.Mark(domain.Waiting, "")
	} else {
		if err := s.engine.Execute(ctx, id, command); err != nil {
			return err
		}
		switch command {
		case Pause:
			task.Mark(domain.Paused, "")
		case Resume:
			task.Mark(domain.Waiting, "")
		case Remove:
			s.tasks = slices.Delete(s.tasks, index, index+1)
		}
	}
	return s.persist(ctx)
}
func apply(task *domain.Task, o Observation) {
	task.Observe(domain.Observation{Status: domain.Status(o.Status), Name: o.Name, Path: o.Path, Error: o.Error, Failure: o.Failure, Total: o.Total, Completed: o.Completed, Speed: o.Speed})
}
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	observations, err := s.engine.Observe(ctx)
	if err != nil {
		return err
	}
	byID := map[string]Observation{}
	for _, o := range observations {
		byID[o.ID] = o
	}
	transition := false
	for i := range s.tasks {
		if o, ok := byID[s.tasks[i].Record().ID]; ok {
			before := s.tasks[i].Record()
			if before.Failure == domain.SourceUnverified {
				continue
			}
			apply(&s.tasks[i], o)
			after := s.tasks[i].Record()
			s.dirty = s.dirty || before != after
			transition = transition || before.Status != after.Status
		}
	}
	if s.dirty && (transition || time.Since(s.lastSave) > 3*time.Second) {
		return s.persist(ctx)
	}
	return nil
}
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	directory := s.preferences.DefaultDirectory
	if directory == "" {
		directory = s.directory
	}
	snap := Snapshot{Directory: directory, Preferences: s.preferences}
	for _, task := range s.tasks {
		r := task.Record()
		dto := TaskDTO{ID: r.ID, URL: r.URL, Name: r.Name, Directory: r.Directory, Path: r.Path, Status: string(r.Status), Error: task.FailureDescription(), Total: r.Total, Completed: r.Completed, Speed: r.Speed, Progress: task.Progress(), CanPause: task.Allows(Pause), CanResume: task.Allows(Resume), CanRetry: task.Allows(Retry), CanRestart: task.Allows(Restart)}
		snap.Tasks = append(snap.Tasks, dto)
		if r.Status == domain.Active {
			snap.Active++
			snap.Speed += r.Speed
		}
	}
	return snap
}
func (s *Service) persist(ctx context.Context) error {
	state := State{Version: 1, Directory: s.directory, Preferences: s.preferences}
	for _, task := range s.tasks {
		r := task.Record()
		r.Speed = 0
		state.Tasks = append(state.Tasks, r)
	}
	if err := s.repo.Save(ctx, state); err != nil {
		s.dirty = true
		return fmt.Errorf("保存任务失败：%w", err)
	}
	s.dirty = false
	s.lastSave = time.Now()
	return nil
}
func (s *Service) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persist(ctx)
}

func (s *Service) SetDefaultDirectory(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("请选择绝对路径作为默认保存目录")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.preferences
	next.DefaultDirectory = filepath.Clean(path)
	return s.savePreferences(ctx, next)
}

func (s *Service) SetStaticBackground(ctx context.Context, still bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.preferences
	next.StaticBackground = still
	return s.savePreferences(ctx, next)
}

func (s *Service) SetTheme(ctx context.Context, mode ThemeMode) error {
	if !mode.valid() {
		return errors.New("未知的主题选项")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.preferences
	next.Theme = mode
	return s.savePreferences(ctx, next)
}

// Called under the service lock, alongside task and progress persistence.
func (s *Service) savePreferences(ctx context.Context, next Preferences) error {
	previous := s.preferences
	s.preferences = next
	if err := s.persist(ctx); err != nil {
		s.preferences = previous
		return err
	}
	return nil
}
