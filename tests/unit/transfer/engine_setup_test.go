package transfer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
)

type engineSetupStub struct {
	findErr  error
	attempts int
}

func (s *engineSetupStub) Find() (string, error) { return "/existing", s.findErr }
func (s *engineSetupStub) Install(context.Context) (string, error) {
	s.attempts++
	if s.attempts == 1 {
		return "", errors.New("offline")
	}
	return "/installed", nil
}

func TestEnginePreparationWaitsForClickAndAllowsRetry(t *testing.T) {
	setup := &engineSetupStub{findErr: application.ErrEngineMissing}
	clicks := make(chan struct{}, 1)
	offers := 0
	path, err := application.PrepareEngine(t.Context(), setup, clicks, func(reason error) {
		if offers != setup.attempts || (offers == 0) != (reason == nil) {
			t.Fatalf("offer %d, attempts %d, reason %v", offers, setup.attempts, reason)
		}
		offers++
		clicks <- struct{}{}
	})
	if err != nil || path != "/installed" || offers != 2 {
		t.Fatalf("path=%q offers=%d err=%v", path, offers, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	setup.attempts = 0
	_, err = application.PrepareEngine(ctx, setup, clicks, func(error) { cancel() })
	if !errors.Is(err, context.Canceled) || setup.attempts != 0 {
		t.Fatal("quit while awaiting consent must not install", err)
	}
}

func TestEnginePreparationUsesExistingOrReportsLookupFailure(t *testing.T) {
	for _, reason := range []error{nil, errors.New("invalid explicit path")} {
		setup := &engineSetupStub{findErr: reason}
		path, err := application.PrepareEngine(t.Context(), setup, nil, func(error) { t.Fatal("unexpected offer") })
		if err != reason || (err == nil && path != "/existing") || setup.attempts != 0 {
			t.Fatalf("%q %v", path, err)
		}
	}
}

func TestWorkspaceOffersEngineDownloadInPlace(t *testing.T) {
	clicks := 0
	v := presentation.New(nil, presentation.Actions{DownloadEngine: func() { clicks++ }})
	v.OfferEngineDownload(nil)
	tt := interactionScene(v, 800, 560)
	if _, offered := tt.AccessByRole(ui.RoleButton, "下载引擎"); !offered || tt.HasText("新建下载") || clicks != 0 {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("下载引擎"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("下载中…") || v.DialogOpen || clicks != 1 {
		t.Fatal(tt.Texts(), clicks)
	}
	_ = tt.Click("下载中…")
	if clicks != 1 {
		t.Fatal("duplicate install")
	}
	v.OfferEngineDownload(application.ErrEngineDownload)
	tt.Frame()
	if _, offered := tt.AccessByRole(ui.RoleButton, "下载引擎"); !offered || !tt.HasText("无法下载引擎") {
		t.Fatal(tt.Texts())
	}
	v.FinishEngineStartup(nil, application.Snapshot{}, application.ErrEngineStart)
	tt.Frame()
	if _, offered := tt.AccessByRole(ui.RoleButton, "下载引擎"); offered || !tt.HasText("aria2 启动失败，请查看数据目录内的 engine.log") {
		t.Fatal(tt.Texts())
	}
	v.FinishEngineStartup(&application.Service{}, application.Snapshot{}, nil)
	tt.Frame()
	if err := tt.Click("新建下载"); err != nil || !v.DialogOpen {
		t.Fatal(tt.Texts(), err)
	}
}
