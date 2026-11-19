//go:build integration

package transfer_test

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func assertPrivateTaskFile(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		t.Fatalf("expected a DACL on the task file: %v", err)
	}
	for i := range uint32(acl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			t.Fatal(err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask&(windows.FILE_READ_DATA|windows.GENERIC_READ|windows.GENERIC_ALL) == 0 {
			continue
		}
		// SidStart is the first word of the variable-length SID stored in the ACE.
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sid.IsWellKnown(windows.WinWorldSid) || sid.IsWellKnown(windows.WinAuthenticatedUserSid) || sid.IsWellKnown(windows.WinBuiltinUsersSid) {
			t.Errorf("task file grants read access to %s", sid)
		}
	}
}
