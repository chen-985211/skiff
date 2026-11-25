package platform_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/platform/diagnostics"
)

func TestStructuredDiagnosticRecordIncludesOnlySafeFields(t *testing.T) {
	var output bytes.Buffer
	logger := diagnostics.New(&output)
	op := logger.Start("transfer.engine", "start")
	logger.Failure(op, "ENGINE_START_FAILED", false)

	var entry struct {
		Timestamp     time.Time
		Level         string
		Scope         string
		Operation     string
		Outcome       string
		DurationMS    *int64 `json:"durationMs"`
		CorrelationID string `json:"correlationId"`
		Error         struct {
			Code       string
			IsExpected bool
		}
	}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Timestamp.IsZero() || entry.DurationMS == nil || *entry.DurationMS < 0 || entry.CorrelationID == "" {
		t.Fatalf("incomplete event: %s", output.String())
	}
	if entry.Scope != "transfer.engine" || entry.Operation != "start" || entry.Outcome != "failure" || entry.Level != "error" || entry.Error.Code != "ENGINE_START_FAILED" || entry.Error.IsExpected {
		t.Fatalf("invalid failure event: %s", output.String())
	}
	if len(strings.Split(strings.TrimSpace(output.String()), "\n")) != 1 {
		t.Fatalf("expected one JSON line: %s", output.String())
	}
}

func TestDiagnosticFailureDoesNotIncludeRawCause(t *testing.T) {
	var output bytes.Buffer
	logger := diagnostics.New(&output)
	op := logger.Start("desktop", "run")
	cause := errors.New("open /home/user/private?key=synthetic-secret")
	logger.Unexpected(op, cause)

	if result := output.String(); strings.Contains(result, "synthetic-secret") || strings.Contains(result, "/home/user/private") || !strings.Contains(result, "UNEXPECTED_ERROR") {
		t.Fatalf("unexpected fields in failure record: %s", result)
	}
}

func TestTransferDiagnosticUsesOnlyTheCodeOfAWrappedFailure(t *testing.T) {
	var output bytes.Buffer
	logger := diagnostics.New(&output)
	cause := errors.Join(application.ErrEngineConnection, errors.New("private repository data"))
	logger.TransferFailure(logger.Start("transfer.queue", "save"), application.Wrap(application.ErrSaveQueue, cause))
	var entry struct {
		Error struct{ Code application.ErrorCode }
	}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("expected one diagnostic record: %v", err)
	}
	if entry.Error.Code != application.ErrSaveQueue || strings.Contains(output.String(), "private repository data") {
		t.Fatalf("wrapped cause leaked or changed the diagnostic code: %s", output.String())
	}
}
