package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
)

type Repository struct{ Path string }

func (r Repository) Load(ctx context.Context) (application.State, error) {
	if err := ctx.Err(); err != nil {
		return application.State{}, err
	}
	data, err := os.ReadFile(r.Path)
	if errors.Is(err, os.ErrNotExist) {
		return application.State{Version: 1}, nil
	}
	if err != nil {
		return application.State{}, application.Wrap(application.ErrReadQueue, err)
	}
	var state application.State
	if err := json.Unmarshal(data, &state); err != nil {
		return state, application.Wrap(application.ErrCorruptQueue, err)
	}
	if state.Version != 1 {
		return state, fmt.Errorf("%w: version %d", application.ErrQueueVersion, state.Version)
	}
	return state, nil
}
func (r Repository) Save(ctx context.Context, state application.State) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(r.Path, data)
}

// WriteAtomic replaces a private file only after a complete, synced write.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".skiff-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
