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
	Prioritize        = domain.Prioritize
	ResumeUnsupported = domain.ResumeUnsupported
	Pause             = domain.Pause
	Resume            = domain.Resume
	Retry             = domain.Retry
	Restart           = domain.Restart
	Remove            = domain.Remove
)

type SourceInfo struct {
	Digest string
	domain.SourceVersion
	Filename   string
	RangeBytes int64
}

type NetworkSettings = domain.NetworkSettings
type DiskSettings = domain.DiskSettings
type SeedPolicy = domain.SeedPolicy
type TransferSpec struct {
	Seed                       SeedPolicy
	PeerLimit                  int
	BT                         BTSettings
	Trackers                   string
	TorrentHash, SelectedFiles string
	Network                    NetworkSettings `json:",omitzero"`
	DownloadLimit              int64           `json:",omitempty"`
	Digest                     string
	Disk                       DiskSettings
	Headers                    string
	FTPUser, FTPPassword       string
	ConnectionLimit            int
	Version                    domain.SourceVersion
	ID, URL, Directory         string
	OutputName                 string
	Paused                     bool
}
type Observation struct {
	Seeding                       bool
	LocalRoute                    bool
	HTTPRejected                  bool
	NextInQueue                   bool
	Connections                   int
	ID, Status, Name, Path, Error string
	Failure                       string
	Total, Completed, Speed       int64
}
type OutputCandidate struct{ ID, Name string }

type Engine interface {
	SetDownloadLimit(context.Context, string, int64) error
	ProtectCheckpoints(context.Context, string, []OutputCandidate) (map[string]string, error)
	SetConcurrency(context.Context, int) error
	HasPartial(context.Context, domain.Record) (bool, error)
	InspectRequest(context.Context, TransferSpec, bool) (SourceInfo, error)
	Enqueue(context.Context, TransferSpec) error
	Execute(context.Context, string, Command) error
	Observe(context.Context) ([]Observation, error)
}
type State struct {
	BTSubscriptions []BTSubscription `json:",omitempty"`
	Preferences     Preferences
	Version         int
	Directory       string
	Tasks           []domain.Record
}
type Repository interface {
	Load(context.Context) (State, error)
	Save(context.Context, State) error
}
type TaskDTO struct {
	Seeding                                       bool
	PeerLimit                                     int
	TorrentHash                                   string
	DownloadLimit                                 int64 `json:",omitempty"`
	MirrorSource                                  string
	NeedsFTPLogin                                 bool
	CanPrioritize, NextInQueue                    bool
	Connections, ConnectionLimit                  int
	ID, URL, Name, Directory, Path, Status, Error string
	Failure                                       string
	Total, Completed, Speed                       int64
	Progress                                      float64
	CanPause, CanResume, CanRetry                 bool
	CanRestart                                    bool
}
type Snapshot struct {
	BTSubscriptions []BTSubscription
	Preferences     Preferences
	Tasks           []TaskDTO
	Directory       string
	Speed           int64
	Active          int
}

// Service is the single serialized entry point for download intent and progress.
// Call it from workers, never from the UI thread.
type Service struct {
	subscriptions      map[string]BTSubscription
	subscriptionMu     sync.Mutex
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
	source, headers := r.URL, r.Headers
	digest := ""
	if r.MirrorSource != "" {
		source = r.MirrorSource
		digest = r.Digest
		if !sameOrigin(r.URL, source) {
			headers = ""
		}
	}
	return TransferSpec{Seed: r.Seed, PeerLimit: r.PeerLimit, TorrentHash: r.TorrentHash, SelectedFiles: r.SelectedFiles, Network: r.Network, DownloadLimit: r.DownloadLimit, Digest: digest, Disk: r.Disk, Headers: headers, FTPUser: r.FTPUser, FTPPassword: r.FTPPassword, ID: r.ID, URL: source, Directory: r.Directory, OutputName: name, Version: r.Version, ConnectionLimit: r.ConnectionLimit, Paused: paused}
}
func (s *Service) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	s.preferences = state.Preferences
	s.subscriptions = make(map[string]BTSubscription)
	for _, sub := range state.BTSubscriptions {
		if sub.Error != "" && sub.Failure == "" {
			sub.Failure = ErrUnexpected
		}
		sub.Error = ""
		s.subscriptions[sub.URL] = sub
	}
	if s.preferences.Concurrency < 1 || s.preferences.Concurrency > MaxConcurrency {
		s.preferences.Concurrency = DefaultConcurrency
	}
	if err := s.engine.SetDownloadLimit(ctx, "", s.preferences.DownloadLimit); err != nil {
		return err
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
		if o, ok := found[r.ID]; ok && (r.Headers != "" || o.LocalRoute) {
			// Loopback credential routes have fresh capabilities each launch.
			if err := s.engine.Execute(ctx, r.ID, Remove); err != nil {
				return err
			}
			delete(found, r.ID)
		}
		if o, ok := found[r.ID]; ok {
			if o.Status != "complete" && o.Status != "error" {
				if err := s.engine.SetDownloadLimit(ctx, r.ID, r.DownloadLimit); err != nil {
					return err
				}
			}
			apply(task, o)
			if task.Allows(domain.Pause) {
				if err := s.engine.Execute(ctx, r.ID, Pause); err != nil {
					return err
				}
				task.Mark(domain.Paused, "")
			}
		} else {
			err := s.enqueue(ctx, transferSpec(r, true))
			if err != nil {
				task.MarkFailure(CodeOf(err), err.Error())
			} else {
				task.Mark(domain.Paused, "")
			}
		}
	}
	return s.persist(ctx)
}
func (s *Service) Execute(ctx context.Context, id string, command Command) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := slices.IndexFunc(s.tasks, func(t domain.Task) bool { return t.Record().ID == id })
	if index < 0 {
		return ErrTaskNotFound
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
			failures = append(failures, fmt.Errorf("%s: %w", record.Name, err))
		}
	}
	return errors.Join(failures...)
}

// execute requires s.mu and persists each accepted change before returning.
func (s *Service) execute(ctx context.Context, index int, command Command) error {
	task := &s.tasks[index]
	if !task.Allows(command) {
		return ErrCommandUnavailable
	}
	r := task.Record()
	id := r.ID
	if command == Prioritize {
		return s.engine.Execute(ctx, id, Prioritize)
	}
	// A paused task already owns an aria2 checkpoint. Resume it in place.
	// A failed task with no bytes can adopt the source's current version on retry.
	if command == Retry && r.TorrentHash == "" && r.Completed == 0 {
		partial, err := s.engine.HasPartial(ctx, r)
		if err != nil {
			return err
		}
		if !partial {
			info, _ := s.engine.InspectRequest(ctx, transferSpec(r, false), r.RequestedConnections != 1)
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
		version, _ := s.engine.InspectRequest(ctx, transferSpec(r, false), false)
		task.RestartAs(outputName)
		task.SetVersion(version.SourceVersion)
		// Commit the new target before any external effect. The previous partial
		// file and its control file are never removed or overwritten.
		if err := s.persist(ctx); err != nil {
			*task = previous
			return err
		}
		if err := s.engine.Execute(ctx, id, Restart); err != nil {
			task.MarkFailure(CodeOf(err), err.Error())
			_ = s.persist(ctx)
			return err
		}
		if err := s.enqueue(ctx, transferSpec(task.Record(), false)); err != nil {
			task.MarkFailure(CodeOf(err), err.Error())
			_ = s.persist(ctx)
			return err
		}
	} else if command == Retry {
		if err := s.engine.Execute(ctx, id, Retry); err != nil {
			return err
		}
		if err := s.enqueue(ctx, transferSpec(task.Record(), false)); err != nil {
			task.MarkFailure(CodeOf(err), err.Error())
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
	if task.Record().TorrentHash != "" {
		o.Name, o.Path = "", ""
	}
	task.Observe(domain.Observation{Seeding: o.Seeding, NextInQueue: o.NextInQueue, Connections: o.Connections, Status: domain.Status(o.Status), Name: o.Name, Path: o.Path, Error: o.Error, Failure: o.Failure, Total: o.Total, Completed: o.Completed, Speed: o.Speed})
}
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	rejected, err := s.refresh(ctx)
	var fallbacks []domain.Record
	for _, t := range s.tasks {
		r := t.Record()
		if r.Mirrors != "" && r.Status == domain.Failed && (r.Failure == domain.SourceUnavailable || r.Failure == ResumeUnsupported || r.Failure == domain.SourceChanged) {
			fallbacks = append(fallbacks, r)
		}
	}
	s.mu.Unlock()
	if err == nil {
		for _, r := range fallbacks {
			if err := s.tryMirror(ctx, r); err != nil {
				return err
			}
		}
	}
	if err != nil || len(rejected) == 0 {
		return err
	}
	// aria2 can collapse a conditional HTTP rejection to "No URI available".
	// Diagnose only failed transfers, outside the queue lock.
	urls := make([]TransferSpec, len(rejected))
	for i, r := range rejected {
		urls[i] = transferSpec(r, false)
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
	snap := Snapshot{BTSubscriptions: s.btSources(), Directory: directory, Preferences: s.preferences}
	for _, task := range s.tasks {
		r := task.Record()
		dto := TaskDTO{Seeding: task.Seeding(), PeerLimit: r.PeerLimit, TorrentHash: r.TorrentHash, DownloadLimit: r.DownloadLimit, MirrorSource: r.MirrorSource, NeedsFTPLogin: r.Failure == domain.FTPLoginRequired, CanPrioritize: task.Allows(Prioritize), NextInQueue: task.NextInQueue(), Connections: task.Connections(), ConnectionLimit: r.ConnectionLimit, ID: r.ID, URL: r.URL, Name: r.Name, Directory: r.Directory, Path: r.Path, Status: string(r.Status), Error: r.Error, Failure: r.Failure, Total: r.Total, Completed: r.Completed, Speed: r.Speed, Progress: task.Progress(), CanPause: task.Allows(Pause), CanResume: task.Allows(Resume), CanRetry: task.Allows(Retry), CanRestart: task.Allows(Restart)}
		snap.Tasks = append(snap.Tasks, dto)
		if r.Status == domain.Active {
			snap.Active++
			snap.Speed += r.Speed
		}
	}
	return snap
}
func (s *Service) persist(ctx context.Context) error {
	state := State{BTSubscriptions: s.btSources(), Version: 1, Directory: s.directory, Preferences: s.preferences}
	for _, task := range s.tasks {
		r := task.Record()
		r.Speed = 0
		state.Tasks = append(state.Tasks, r)
	}
	if err := s.repo.Save(ctx, state); err != nil {
		s.dirty = true
		return Wrap(ErrSaveQueue, err)
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

func (s *Service) RetryFTP(ctx context.Context, id, user, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.tasks, func(t domain.Task) bool { return t.Record().ID == id })
	if i < 0 || s.tasks[i].Record().Failure != domain.FTPLoginRequired {
		return ErrFTPLoginUnneeded
	}
	if strings.TrimSpace(user) == "" || strings.ContainsAny(user+password, "\r\n\x00") {
		return ErrInvalidFTPLogin
	}
	previous := s.tasks[i]
	s.tasks[i].SetFTPLogin(user, password)
	if err := s.persist(ctx); err != nil {
		s.tasks[i] = previous
		return err
	}
	return s.execute(ctx, i, Retry)
}
