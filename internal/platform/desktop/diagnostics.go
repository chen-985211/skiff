package desktop

import (
	"context"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/platform/diagnostics"
)

type observedInstallation struct {
	application.EngineInstallation
	logger *diagnostics.Logger
}

func (i observedInstallation) Install(ctx context.Context) (string, error) {
	operation := i.logger.Start("transfer.engine", "install")
	path, err := i.EngineInstallation.Install(ctx)
	if ctx.Err() == nil {
		i.logger.TransferFailure(operation, err)
	}
	return path, err
}
