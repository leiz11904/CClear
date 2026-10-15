package appinfo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersionIsNotMarkedWindowsVerified(t *testing.T) {
	if Version != "2.7.0" {
		t.Fatalf("version %s", Version)
	}
	if WindowsNativeVerified {
		t.Fatal("Windows native verification is marked done without a recorded Windows run")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	app, err := os.ReadFile(filepath.Join(root, "assets", "cclear.manifest"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(app), `level="asInvoker"`) || strings.Contains(string(app), "requireAdministrator") {
		t.Fatal("cleaner manifest is not asInvoker")
	}
	setup, err := os.ReadFile(filepath.Join(root, "assets", "setup.manifest"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(setup), `level="requireAdministrator"`) {
		t.Fatal("installer manifest does not request administrator")
	}
}
