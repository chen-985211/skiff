package aria2

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows"
)

func directoryWriteAccess(dir string) error {
	// Use the extended Win32 spelling for absolute paths, including long UNC paths.
	switch {
	case strings.HasPrefix(dir, `\\?\`), strings.HasPrefix(dir, `\\.\`):
	case strings.HasPrefix(dir, `\\`):
		dir = `\\?\UNC\` + dir[2:]
	default:
		dir = `\\?\` + dir
	}
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	// FILE_WRITE_DATA on a directory grants permission to create child files.
	handle, err := windows.CreateFile(path, windows.FILE_WRITE_DATA, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	return windows.CloseHandle(handle)
}

func isVolumeFull(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) || errors.Is(err, windows.ERROR_HANDLE_DISK_FULL)
}
