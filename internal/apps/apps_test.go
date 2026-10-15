package apps

import (
	"context"
	"strings"
	"testing"
)

func TestListInstalledDoesNotInventApps(t *testing.T) {
	rep := ListInstalled(context.Background())
	if !strings.Contains(rep.Note, "卸载") && !strings.Contains(rep.Note, "Windows") {
		t.Fatal(rep.Note)
	}
}

func TestUninstallCommandBlocksScripts(t *testing.T) {
	_, _, err := UninstallCommand(App{Uninstall: `cmd.exe /c del C:\app`})
	if err == nil {
		t.Fatal("script uninstall was allowed")
	}
	exe, args, err := UninstallCommand(App{Uninstall: `"C:\Program Files\App\Uninstall.exe" /S`})
	if err != nil || exe != `C:\Program Files\App\Uninstall.exe` || args != "/S" {
		t.Fatalf("%s %s %v", exe, args, err)
	}
}
