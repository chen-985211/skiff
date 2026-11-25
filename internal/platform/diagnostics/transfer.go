package diagnostics

import (
	"context"

	"skiff/internal/contexts/transfer/application"
)

// ObservedTransfer records diagnostics for manual and scheduled subscription refreshes.
type ObservedTransfer struct {
	*application.Service
	logger *Logger
}

func (l *Logger) ObserveTransfer(service *application.Service) *ObservedTransfer {
	return &ObservedTransfer{Service: service, logger: l}
}

func (s *ObservedTransfer) RefreshBTSubscriptions(ctx context.Context, force bool) error {
	op := s.logger.Start("transfer.tracker", "refresh_subscriptions")
	err := s.Service.RefreshBTSubscriptions(ctx, force)
	if ctx.Err() == nil {
		s.logger.TransferFailure(op, err)
	}
	return err
}

func (l *Logger) TransferFailure(op Operation, err error) {
	if err == nil {
		return
	}
	failures := []error{err}
	if batch, ok := err.(*application.OperationFailures); ok {
		failures = batch.Unwrap()
	}
	seen := make(map[application.ErrorCode]bool, len(failures))
	for _, failure := range failures {
		code := application.CodeOf(failure)
		if !seen[code] {
			l.Failure(op, string(code), application.IsExpected(code))
			seen[code] = true
		}
	}
}
