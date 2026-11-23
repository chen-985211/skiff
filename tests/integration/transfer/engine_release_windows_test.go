//go:build integration && engine_release

package transfer_test

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestNativeEngineReleaseDownloadsFile(t *testing.T) {
	// Explorer launches Skiff without a console. Recreate that condition even
	// when the test runner itself was started from a terminal.
	if os.Getenv("SKIFF_TEST_DESKTOP_PROCESS") != "1" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestNativeEngineReleaseDownloadsFile$")
		cmd.Env = append(os.Environ(), "SKIFF_TEST_DESKTOP_PROCESS=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("desktop process: %v\n%s", err, output)
		}
		return
	}

	downloadWithNativeEngineRelease(t)
	// The real downloaded worker stays alive until test cleanup. A detached
	// worker has no console for the desktop user to close.
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	attached, _, err := kernel.NewProc("AttachConsole").Call(uintptr(engineChildPID(t)))
	if attached != 0 {
		kernel.NewProc("FreeConsole").Call()
		t.Fatal("aria2c.exe owns a console after desktop startup")
	}
	if !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		t.Fatalf("checking the engine console: %v", err)
	}
}

func engineChildPID(t *testing.T) uint32 {
	t.Helper()
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(snapshot)
	var proc windows.ProcessEntry32
	proc.Size = uint32(unsafe.Sizeof(proc))
	for err = windows.Process32First(snapshot, &proc); err == nil; err = windows.Process32Next(snapshot, &proc) {
		if proc.ParentProcessID == uint32(os.Getpid()) && strings.EqualFold(windows.UTF16ToString(proc.ExeFile[:]), "aria2c.exe") {
			return proc.ProcessID
		}
	}
	t.Fatalf("could not find the running aria2c.exe child: %v", err)
	return 0
}
