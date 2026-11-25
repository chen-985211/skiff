//go:build unix

package aria2

import (
	"errors"

	"golang.org/x/sys/unix"
)

func directoryWriteAccess(dir string, _ bool) error {
	return unix.Access(dir, unix.W_OK|unix.X_OK)
}

func isVolumeFull(err error) bool {
	return errors.Is(err, unix.ENOSPC)
}
