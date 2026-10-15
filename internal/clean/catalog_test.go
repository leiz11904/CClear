package clean

import (
	"strings"
	"testing"
	"time"

	"cclear/internal/winpath"
)

func sampleEnv() Env {
	return Env{
		SystemDrive:  "C:",
		SystemRoot:   `C:\Windows`,
		LocalAppData: `C:\Users\ada\AppData\Local`,
		ProgramData:  `C:\ProgramData`,
		Temp:         `C:\Users\ada\AppData\Local\Temp`,
		UserProfile:  `C:\Users\ada`,
	}
}

func TestCategoryRootsStayOnAllowlist(t *testing.T) {
	cats := Categories(sampleEnv())
	if len(cats) < 10 {
		t.Fatalf("categories = %d", len(cats))
	}
	for _, cat := range cats {
		if cat.Name == "" || cat.ID == "" {
			t.Fatalf("incomplete category %+v", cat)
		}
		for _, root := range cat.Roots {
			if err := winpath.ValidateRoot(root, "C:"); err != nil {
				t.Errorf("%s root %s: %v", cat.ID, root, err)
			}
			lower := strings.ToLower(root)
			for _, blocked := range []string{`\downloads`, `\documents`, `\desktop`, `\system32`, `\winsxs`, `\datastore`} {
				if strings.Contains(lower, blocked) {
					t.Errorf("%s includes protected path %s", cat.ID, root)
				}
			}
		}
	}
}

func TestDownloadsCannotBecomeTemp(t *testing.T) {
	env := sampleEnv()
	env.Temp = `C:\Users\ada\Downloads`
	for _, cat := range Categories(env) {
		for _, root := range cat.Roots {
			if strings.Contains(strings.ToLower(root), `downloads`) {
				t.Fatalf("downloads entered catalog via %s: %s", cat.ID, root)
			}
		}
		if cat.ID == "user-temp" {
			if len(cat.Roots) != 1 || !strings.EqualFold(cat.Roots[0], `C:\Users\ada\AppData\Local\Temp`) {
				t.Fatalf("user temp roots = %#v", cat.Roots)
			}
		}
		if (cat.Risk >= 2 || cat.ID == "recycle" || cat.ID == "windowsold") && cat.DefaultOn {
			t.Fatalf("%s is selected by default", cat.ID)
		}
		if cat.ID == "windowsold" && (cat.Special != "windowsold" || cat.RemoveRoot) {
			t.Fatal("windows.old would be deleted recursively")
		}
		if (cat.ID == "user-temp" || cat.ID == "windows-temp") && cat.MinAge < 72*time.Hour {
			t.Fatalf("%s age = %s", cat.ID, cat.MinAge)
		}
		if cat.ID == "wer" && cat.DefaultOn {
			t.Fatal("error reports start selected")
		}
		if (cat.ID == "winupdate" || cat.ID == "dism") && cat.DefaultOn {
			t.Fatalf("%s is selected by default", cat.ID)
		}
	}
}

func TestOtherDriveIsDropped(t *testing.T) {
	env := sampleEnv()
	env.Temp = `D:\Temp`
	for _, cat := range Categories(env) {
		if cat.ID != "user-temp" {
			continue
		}
		for _, root := range cat.Roots {
			if strings.HasPrefix(strings.ToUpper(root), `D:`) {
				t.Fatal(root)
			}
		}
	}
}
