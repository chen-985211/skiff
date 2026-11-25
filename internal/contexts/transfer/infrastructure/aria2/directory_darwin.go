package aria2

import (
	"errors"

	"golang.org/x/sys/unix"
)

func directoryWriteAccess(dir string, createSubdirectory bool) error {
	// These extended access flags in Darwin's sys/unistd.h check each ACL right separately.
	const addFile, search, addSubdirectory = 1 << 10, 1 << 11, 1 << 13
	mode := uint32(addFile | search)
	if createSubdirectory {
		mode = addSubdirectory | search
	}
	return unix.Faccessat(unix.AT_FDCWD, dir, mode, unix.AT_EACCESS)
}

func isVolumeFull(err error) bool {
	return errors.Is(err, unix.ENOSPC)
}
