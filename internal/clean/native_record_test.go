//go:build !windows

package clean

import "testing"

// TestWindowsNativeSuiteWasNotRunHere records that this process is not Windows.
// A pass is not a pass of the NTFS suite.
func TestWindowsNativeSuiteWasNotRunHere(t *testing.T) {
	t.Log("NOT EXECUTED: NTFS junction, symbolic link, mount point, sharing violation, file replacement, ACL, UAC prompt, Windows Update install, real browser profiles, DISM and cleanmgr")
}
