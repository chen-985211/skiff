package application

import (
	"context"
	"errors"
	"path/filepath"
	"skiff/internal/contexts/transfer/domain"
)

// A staged move retains both copies until the new library location is durable.
type TorrentMove interface{ Finish(bool) error }
type TorrentFiles interface {
	VerifyFiles(context.Context, string, string) error
	CopyFiles(context.Context, string, string, string) (TorrentMove, error)
}

func (s *Service) torrentRecord(id string) (domain.Record, error) {
	for _, t := range s.tasks {
		r := t.Record()
		if r.ID == id && r.TorrentHash != "" {
			return r, nil
		}
	}
	return domain.Record{}, ErrTorrentNotFound
}
func (s *Service) VerifyTorrentFiles(ctx context.Context, id string) error {
	s.mu.Lock()
	r, err := s.torrentRecord(id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	lib, ok := s.engine.(TorrentFiles)
	if !ok {
		return ErrVerifyUnsupported
	}
	return lib.VerifyFiles(ctx, r.TorrentHash, r.Directory)
}

func (s *Service) UseTorrentFiles(ctx context.Context, id, dir string, move, seed bool) (err error) {
	if !filepath.IsAbs(dir) {
		return ErrTorrentDirectory
	}
	dir = filepath.Clean(dir)
	s.mu.Lock()
	r, err := s.torrentRecord(id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if r.Status == domain.Active || r.Status == domain.Waiting {
		return ErrPauseBeforeMoving
	}
	lib, ok := s.engine.(TorrentFiles)
	if !ok {
		return ErrVerifyUnsupported
	}
	var staged TorrentMove
	adopted := false
	if move && dir != r.Directory {
		staged, err = lib.CopyFiles(ctx, r.TorrentHash, r.Directory, dir)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, staged.Finish(adopted)) }()
	}
	if err = lib.VerifyFiles(ctx, r.TorrentHash, dir); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.torrentRecord(id)
	if err != nil {
		return err
	}
	if current != r {
		return ErrTaskChanged
	}
	for i := range s.tasks {
		task := &s.tasks[i]
		entry := task.Record()
		if entry.ID != id && entry.TorrentHash == r.TorrentHash && entry.Directory == dir {
			return ErrDestinationInUse
		}
	}
	for i := range s.tasks {
		task := &s.tasks[i]
		if task.Record().ID != id {
			continue
		}
		old := *task
		if err = s.engine.Execute(ctx, id, Remove); err != nil {
			return err
		}
		r.Directory, r.Path = dir, filepath.Join(dir, r.Name)
		r.Status, r.Error, r.Failure, r.Completed = domain.Complete, "", "", r.Total
		if seed {
			r.Seed = s.preferences.BT.Seed
			r.Seed.Enabled = true
			r.Status = domain.Waiting
		}
		*task = domain.Restore(r)
		if err = s.persist(ctx); err != nil {
			*task = old
			if current.Status == domain.Paused {
				err = errors.Join(err, s.enqueue(ctx, transferSpec(current, true)))
			}
			return err
		}
		adopted = true
		if seed {
			if err = s.enqueue(ctx, transferSpec(r, false)); err != nil {
				task.MarkFailure(CodeOf(err), err.Error())
				return errors.Join(err, s.persist(ctx))
			}
		}
		return nil
	}
	return ErrTorrentNotFound
}
