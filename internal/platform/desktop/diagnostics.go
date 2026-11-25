package desktop

import (
	"context"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/platform/diagnostics"
)

func reportFailure(logger *diagnostics.Logger, operation diagnostics.Operation, err error) {
	if err == nil {
		return
	}
	code := application.CodeOf(err)
	logger.Failure(operation, string(code), application.IsExpected(code))
}

type observedInstallation struct {
	application.EngineInstallation
	logger *diagnostics.Logger
}

func (i observedInstallation) Install(ctx context.Context) (string, error) {
	operation := i.logger.Start("transfer.engine", "install")
	path, err := i.EngineInstallation.Install(ctx)
	if ctx.Err() == nil {
		reportFailure(i.logger, operation, err)
	}
	return path, err
}
