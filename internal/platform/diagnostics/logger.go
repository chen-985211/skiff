package diagnostics

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Logger struct {
	logger *slog.Logger
	runID  string
	nextID atomic.Uint64
}

type Operation struct {
	scope, name, id string
	start           time.Time
}

func Stderr() *Logger {
	return New(os.Stderr)
}

func New(output io.Writer) *Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
		if len(groups) != 0 {
			return attr
		}
		switch attr.Key {
		case slog.TimeKey:
			return slog.Time("timestamp", attr.Value.Time().UTC())
		case slog.LevelKey:
			return slog.String("level", strings.ToLower(attr.Value.String()))
		case slog.MessageKey:
			return slog.Attr{}
		default:
			return attr
		}
	}})
	return &Logger{
		logger: slog.New(handler),
		runID:  strconv.FormatInt(time.Now().UnixNano(), 36),
	}
}

func (l *Logger) Start(scope, name string) Operation {
	return Operation{scope: scope, name: name, start: time.Now(), id: l.runID + "-" + strconv.FormatUint(l.nextID.Add(1), 36)}
}

func (l *Logger) Success(op Operation) {
	l.logger.Info("", op.fields("success")...)
}

func (l *Logger) Failure(op Operation, code string, expected bool) {
	fields := append(op.fields("failure"), slog.Group("error",
		slog.String("code", code),
		slog.Bool("isExpected", expected),
	))
	if expected {
		l.logger.Warn("", fields...)
		return
	}
	l.logger.Error("", fields...)
}

func (l *Logger) Unexpected(op Operation, err error) {
	if err != nil {
		l.Failure(op, "UNEXPECTED_ERROR", false)
	}
}

func (op Operation) fields(outcome string) []any {
	return []any{
		slog.String("scope", op.scope),
		slog.String("operation", op.name),
		slog.String("outcome", outcome),
		slog.Int64("durationMs", time.Since(op.start).Milliseconds()),
		slog.String("correlationId", op.id),
	}
}
