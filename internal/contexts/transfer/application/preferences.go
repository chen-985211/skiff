package application

import (
	"context"
	"path/filepath"
)

const (
	DefaultConcurrency = 10
	MaxConcurrency     = 200
)

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
	BT                BTSettings      `json:",omitzero"`
	Network           NetworkSettings `json:",omitzero"`
	DownloadLimit     int64           `json:",omitempty"`
	Disk              DiskSettings    `json:",omitzero"`
	RecentDirectories [5]string       `json:",omitempty"`
	Concurrency       int             `json:",omitempty"`
	Theme             ThemeMode       `json:",omitempty"`
	DefaultDirectory  string
	StaticBackground  bool
}

func (s *Service) SetDefaultDirectory(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return ErrInvalidDefaultDirectory
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
		return ErrUnknownTheme
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
		return ErrConcurrencyRange
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
		return Wrap(ErrApplyConcurrency, err)
	}
	s.concurrencyPending = false
	return nil
}

func (s *Service) SetDiskSettings(ctx context.Context, d DiskSettings) error {
	if err := d.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.preferences
	next.Disk = d
	return s.savePreferences(ctx, next)
}

func (s *Service) SetNetwork(ctx context.Context, n NetworkSettings) error {
	if err := n.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.preferences
	next.Network = n
	return s.savePreferences(ctx, next)
}
