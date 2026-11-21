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
	DefaultConcurrency = 10
	MaxConcurrency     = 200
	Prioritize         = domain.Prioritize
	ResumeUnsupported  = domain.ResumeUnsupported
	Pause              = domain.Pause
	Resume             = domain.Resume
	Retry              = domain.Retry
	Restart            = domain.Restart
	Remove             = domain.Remove
)

const MaxConnectionLimit = domain.MaxConnectionLimit

type SourceInfo struct {
	domain.SourceVersion
	Filename   string
	RangeBytes int64
}

type TransferSpec struct {
	FTPUser, FTPPassword string
	ConnectionLimit      int
	Version              domain.SourceVersion
	ID, URL, Directory   string
	OutputName           string
	Paused               bool
}
type Observation struct {
	HTTPRejected                  bool
	NextInQueue                   bool
	Connections                   int
	ID, Status, Name, Path, Error string
	Failure                       string
	Total, Completed, Speed       int64
}
type OutputCandidate struct{ ID, Name string }

type Engine interface {
	ProtectCheckpoints(context.Context, string, []OutputCandidate) (map[string]string, error)
	SetConcurrency(context.Context, int) error
	HasPartial(context.Context, domain.Record) (bool, error)
	InspectSource(context.Context, string, bool) (SourceInfo, error)
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
	Concurrency      int       `json:",omitempty"`
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
	NeedsFTPLogin                                 bool
	CanPrioritize, NextInQueue                    bool
	Connections, ConnectionLimit                  int
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
	concurrencyPending bool
	preferences        Preferences
	mu                 sync.Mutex
	engine             Engine
	repo               Repository
	directory          string
	nextID             func() string
	tasks              []domain.Task
	dirty              bool
	lastSave           time.Time
}

func New(engine Engine, repo Repository, directory string, nextID func() string) *Service {
	return &Service{engine: engine, repo: repo, directory: directory, nextID: nextID}
}
func transferSpec(r domain.Record, paused bool) TransferSpec {
	name := r.OutputName
	if r.Path != "" && filepath.Clean(filepath.Dir(r.Path)) == filepath.Clean(r.Directory) {
		name = filepath.Base(r.Path)
	}
	return TransferSpec{FTPUser: r.FTPUser, FTPPassword: r.FTPPassword, ID: r.ID, URL: r.URL, Directory: r.Directory, OutputName: name, Version: r.Version, ConnectionLimit: r.ConnectionLimit, Paused: paused}
}
func (s *Service) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	s.preferences = state.Preferences
	if s.preferences.Concurrency < 1 || s.preferences.Concurrency > MaxConcurrency {
		s.preferences.Concurrency = DefaultConcurrency
	}
	s.concurrencyPending = true
	if err := s.applyConcurrency(ctx); err != nil {
		return err
	}
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
			err := s.engine.Enqueue(ctx, transferSpec(r, true))
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
	return s.AddBatchWithConnections(ctx, text, directory, 0)
}

func (s *Service) AddBatchWithConnections(ctx context.Context, text, directory string, limit int) (BatchResult, error) {
	if limit < 0 || limit > MaxConnectionLimit {
		return BatchResult{}, fmt.Errorf("连接上限应为 1 到 %d，留空使用自动设置", MaxConnectionLimit)
	}
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
	sources := s.inspectBatch(ctx, batch.Sources, limit != 1)
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
		pending[task.Record().URL] = true
		name := sources[i].Filename
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
	s.directory = directory
	// The persistent intent precedes the external effect, so a crash can be reconciled.
	if err := s.persist(ctx); err != nil {
		s.tasks, s.directory = oldTasks, oldDir
		return BatchResult{}, err
	}
	var failures []error
	for i := len(oldTasks); i < len(s.tasks); i++ {
		task := &s.tasks[i]
		if err := s.engine.Enqueue(ctx, transferSpec(task.Record(), false)); err != nil {
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
func (s *Service) inspectBatch(ctx context.Context, sources []string, ranges bool) []SourceInfo {
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
				versions[i], _ = s.engine.InspectSource(ctx, sources[i], ranges)
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
	if command == Prioritize {
		return s.engine.Execute(ctx, id, Prioritize)
	}
	// A paused task already owns an aria2 checkpoint. Resume it in place.
	// A failed task with no bytes can adopt the source's current version on retry.
	if command == Retry && r.Completed == 0 {
		partial, err := s.engine.HasPartial(ctx, r)
		if err != nil {
			return err
		}
		if !partial {
			info, _ := s.engine.InspectSource(ctx, r.URL, r.RequestedConnections != 1)
			previous := *task
			task.SetVersion(info.SourceVersion)
			task.PlanConnections(r.RequestedConnections, info.RangeBytes)
			if err := s.persist(ctx); err != nil {
				*task = previous
				return err
			}
		}
	}
	if command == Restart {
		previous := *task
		name := filepath.Base(r.Name)
		ext := filepath.Ext(name)
		outputName := strings.TrimSuffix(name, ext) + ".restart-" + s.nextID() + ext
		version, _ := s.engine.InspectSource(ctx, r.URL, false)
		task.RestartAs(outputName)
		task.SetVersion(version.SourceVersion)
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
		if err := s.engine.Enqueue(ctx, transferSpec(task.Record(), false)); err != nil {
			task.Mark(domain.Failed, err.Error())
			_ = s.persist(ctx)
			return err
		}
	} else if command == Retry {
		if err := s.engine.Execute(ctx, id, Retry); err != nil {
			return err
		}
		if err := s.engine.Enqueue(ctx, transferSpec(task.Record(), false)); err != nil {
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
	task.Observe(domain.Observation{NextInQueue: o.NextInQueue, Connections: o.Connections, Status: domain.Status(o.Status), Name: o.Name, Path: o.Path, Error: o.Error, Failure: o.Failure, Total: o.Total, Completed: o.Completed, Speed: o.Speed})
}
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	rejected, err := s.refresh(ctx)
	s.mu.Unlock()
	if err != nil || len(rejected) == 0 {
		return err
	}
	// aria2 can collapse a conditional HTTP rejection to "No URI available".
	// Diagnose only failed transfers, outside the queue lock.
	urls := make([]string, len(rejected))
	for i, r := range rejected {
		urls[i] = r.URL
	}
	sources := s.inspectBatch(ctx, urls, false)
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i, r := range rejected {
		if !r.Version.Changed(sources[i].SourceVersion) {
			continue
		}
		for j := range s.tasks {
			if s.tasks[j].Record() == r {
				s.tasks[j].MarkSourceChanged()
				changed = true
				break
			}
		}
	}
	if changed {
		return s.persist(ctx)
	}
	return nil
}

func (s *Service) refresh(ctx context.Context) ([]domain.Record, error) {
	if err := s.applyConcurrency(ctx); err != nil {
		return nil, err
	}
	observations, err := s.engine.Observe(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[string]Observation{}
	for _, o := range observations {
		byID[o.ID] = o
	}
	var rejected []domain.Record
	transition := false
	for i := range s.tasks {
		if o, ok := byID[s.tasks[i].Record().ID]; ok {
			before := s.tasks[i].Record()
			if before.Status == domain.Failed && s.tasks[i].ResumeBlocked() {
				continue
			}
			apply(&s.tasks[i], o)
			after := s.tasks[i].Record()
			if o.HTTPRejected && before.Status != domain.Failed && after.Status == domain.Failed && after.Completed > 0 && after.Version.Verified() {
				rejected = append(rejected, after)
			}
			s.dirty = s.dirty || before != after
			transition = transition || before.Status != after.Status
		}
	}
	if s.dirty && (transition || time.Since(s.lastSave) > 3*time.Second) {
		return rejected, s.persist(ctx)
	}
	return rejected, nil
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
		dto := TaskDTO{NeedsFTPLogin: r.Failure == domain.FTPLoginRequired, CanPrioritize: task.Allows(Prioritize), NextInQueue: task.NextInQueue(), Connections: task.Connections(), ConnectionLimit: r.ConnectionLimit, ID: r.ID, URL: r.URL, Name: r.Name, Directory: r.Directory, Path: r.Path, Status: string(r.Status), Error: task.FailureDescription(), Total: r.Total, Completed: r.Completed, Speed: r.Speed, Progress: task.Progress(), CanPause: task.Allows(Pause), CanResume: task.Allows(Resume), CanRetry: task.Allows(Retry), CanRestart: task.Allows(Restart)}
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

func (s *Service) SetConcurrency(ctx context.Context, limit int) error {
	if limit < 1 || limit > MaxConcurrency {
		return fmt.Errorf("同时下载数应为 1 到 %d", MaxConcurrency)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit != s.preferences.Concurrency {
		next := s.preferences
		next.Concurrency = limit
		if err := s.savePreferences(ctx, next); err != nil {
			return err
		}
		s.concurrencyPending = true
	}
	return s.applyConcurrency(ctx)
}

// The saved preference is authoritative. Retry an interrupted RPC on refresh or open.
func (s *Service) applyConcurrency(ctx context.Context) error {
	if !s.concurrencyPending {
		return nil
	}
	if err := s.engine.SetConcurrency(ctx, s.preferences.Concurrency); err != nil {
		return fmt.Errorf("等待下载引擎应用并发设置：%w", err)
	}
	s.concurrencyPending = false
	return nil
}

func (s *Service) RetryFTP(ctx context.Context, id, user, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.tasks, func(t domain.Task) bool { return t.Record().ID == id })
	if i < 0 || s.tasks[i].Record().Failure != domain.FTPLoginRequired {
		return errors.New("任务无需 FTP 登录")
	}
	if strings.TrimSpace(user) == "" || strings.ContainsAny(user+password, "\r\n\x00") {
		return errors.New("请输入有效的 FTP 账号和密码")
	}
	previous := s.tasks[i]
	s.tasks[i].SetFTPLogin(user, password)
	if err := s.persist(ctx); err != nil {
		s.tasks[i] = previous
		return err
	}
	return s.execute(ctx, i, Retry)
}
