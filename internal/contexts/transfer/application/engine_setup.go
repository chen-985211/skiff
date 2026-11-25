package application

import (
	"context"
	"errors"
)

type EngineInstallation interface {
	Find() (string, error)
	Install(context.Context) (string, error)
}

// PrepareEngine resolves an existing engine immediately. Only a user request
// can initiate an installation or retry while the desktop is running.
func PrepareEngine(ctx context.Context, engine EngineInstallation, requests <-chan struct{}, offer func(error)) (string, error) {
	path, err := engine.Find()
	if !errors.Is(err, ErrEngineMissing) {
		return path, err
	}
	offer(nil)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-requests:
		}
		path, err = engine.Install(ctx)
		if err == nil {
			return path, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		offer(err)
	}
}
