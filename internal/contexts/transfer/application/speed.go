package application

import (
	"context"
	"errors"
	"skiff/internal/contexts/transfer/domain"
	"slices"
)

// A blank ID refers to the whole queue. aria2 enforces the smaller of the
// overall and task ceilings and accepts zero to remove a ceiling.
func (s *Service) SetDownloadLimit(ctx context.Context, id string, rate int64) error {
	if rate < 0 {
		return ErrNegativeRate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.preferences.DownloadLimit
	var task *domain.Task
	running := true
	if id != "" {
		i := slices.IndexFunc(s.tasks, func(t domain.Task) bool { return t.Record().ID == id })
		if i < 0 {
			return ErrTaskNotFound
		}
		task = &s.tasks[i]
		r := task.Record()
		previous = r.DownloadLimit
		running = r.Status != domain.Failed && r.Status != domain.Complete
	}
	if running {
		if err := s.engine.SetDownloadLimit(ctx, id, rate); err != nil {
			return err
		}
	}
	if task == nil {
		s.preferences.DownloadLimit = rate
	} else {
		task.SetDownloadLimit(rate)
	}
	if err := s.persist(ctx); err != nil {
		if task == nil {
			s.preferences.DownloadLimit = previous
		} else {
			task.SetDownloadLimit(previous)
		}
		if running {
			return errors.Join(err, s.engine.SetDownloadLimit(ctx, id, previous))
		}
		return err
	}
	return nil
}
