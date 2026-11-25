package transfer_test

import (
	"context"
	"errors"
	"testing"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

func TestTransferErrorCodesPreserveCauseAndClassifyExpectedErrors(t *testing.T) {
	repo := &fakeRepo{}
	s := service(t, &fakeEngine{}, repo)
	err := s.SetDefaultDirectory(context.Background(), "relative")
	if code := application.CodeOf(err); code != application.ErrInvalidDefaultDirectory || !application.IsExpected(code) {
		t.Fatalf("invalid folder returned code %v", code)
	}

	repo.saveErr = errors.New("disk full")
	err = s.SetDefaultDirectory(context.Background(), t.TempDir())
	if code := application.CodeOf(err); code != application.ErrSaveQueue || application.IsExpected(code) || !errors.Is(err, repo.saveErr) {
		t.Fatalf("save failed with %v, code %v", err, code)
	}
}

func TestEnglishEngineErrorNeverShowsRawCause(t *testing.T) {
	view := presentation.New(nil, presentation.Actions{})
	cause := errors.New("Authorization: Bearer synthetic-secret")
	view.EngineError = application.Wrap(application.ErrEngineConnection, cause)
	scene := interactionScene(view, 800, 560)
	scene.SetLocale("en")
	if !scene.HasText("Could not connect to the download engine") || scene.HasText(view.EngineError.Error()) || scene.HasText(cause.Error()) {
		t.Fatalf("engine failure did not use localized safe copy: %v", scene.Texts())
	}
}
