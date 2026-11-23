//go:build integration && engine_release && !windows

package transfer_test

import "testing"

func TestNativeEngineReleaseDownloadsFile(t *testing.T) {
	downloadWithNativeEngineRelease(t)
}
