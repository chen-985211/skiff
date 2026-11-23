//go:build !windows

package aria2

import "os/exec"

func startEngineProcess(cmd *exec.Cmd) error {
	return cmd.Start()
}
