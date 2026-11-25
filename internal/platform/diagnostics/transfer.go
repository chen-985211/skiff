package diagnostics

import "skiff/internal/contexts/transfer/application"

func (l *Logger) TransferFailure(op Operation, err error) {
	if err == nil {
		return
	}
	failures := []error{err}
	if refresh, ok := err.(*application.SubscriptionRefreshError); ok {
		failures = refresh.Unwrap()
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
