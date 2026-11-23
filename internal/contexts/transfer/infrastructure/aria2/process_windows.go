package aria2

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func startEngineProcess(cmd *exec.Cmd) error {
	// Skiff owns the engine lifetime. A console window opened by Windows
	// would let closing that window terminate aria2 while Skiff is still open.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS}
	return cmd.Start()
}
