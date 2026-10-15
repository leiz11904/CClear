package winpath

import "testing"

func TestClean(t *testing.T) {
	tests := []struct {
		in, out string
		wantErr bool
	}{
		{`C:\Windows\Temp\..\Temp\a.txt`, `C:\Windows\Temp\a.txt`, false},
		{`C:/Windows/Temp/foo`, `C:\Windows\Temp\foo`, false},
		{`C:\Windows\Temp\..\..\Windows\System32\cmd.exe`, `C:\Windows\System32\cmd.exe`, false},
		{`\\?\C:\Windows\Temp\a`, `C:\Windows\Temp\a`, false},
		{`C:\`, `C:\`, false},
		{`\\server\share\a`, "", true},
		{`\\?\UNC\server\share`, "", true},
		{`Temp\a`, "", true},
		{`C:\Windows\..\..\secret`, "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := Clean(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Clean(%q) succeeded with %q", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Clean(%q): %v", tt.in, err)
			continue
		}
		if got != tt.out {
			t.Errorf("Clean(%q) = %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestWithinDoesNotMatchPrefixSibling(t *testing.T) {
	root, _ := Clean(`C:\Windows\Temp`)
	sibling, _ := Clean(`C:\Windows\Tempest\a.txt`)
	child, _ := Clean(`C:\Windows\Temp\a.txt`)
	if Within(sibling, root) {
		t.Fatal("Tempest should not match Temp")
	}
	if !Within(child, root) {
		t.Fatal("child should match")
	}
}

func TestValidateRootAllowlist(t *testing.T) {
	drive := "C:"
	ok := []string{
		`C:\Windows\Temp`,
		`C:\Users\ada\AppData\Local\Temp`,
		`C:\Windows\SoftwareDistribution\Download`,
		`C:\Windows\Logs\CBS`,
		`C:\Windows.old`,
		`C:\Users\ada\AppData\Local\Google\Chrome\User Data\Default\Cache`,
		`C:\Users\ada\AppData\Local\Google\Chrome\User Data\Profile 1\Code Cache`,
		`C:\Users\ada\AppData\Local\Microsoft\Edge\User Data\Default\Service Worker\CacheStorage`,
		`C:\Users\ada\AppData\Local\Mozilla\Firefox\Profiles\abc.default-release\cache2`,
		`C:\Windows\MEMORY.DMP`,
		`C:\Users\ada\AppData\Local\Microsoft\Windows\Explorer`,
	}
	for _, path := range ok {
		if err := ValidateRoot(path, drive); err != nil {
			t.Errorf("ValidateRoot(%s): %v", path, err)
		}
	}
	bad := []string{
		`C:\`,
		`C:\Windows`,
		`C:\Windows\System32`,
		`C:\Windows\System32\drivers`,
		`C:\Windows\WinSxS`,
		`C:\Windows\explorer.exe`,
		`C:\Program Files\App`,
		`C:\Users`,
		`C:\Users\ada`,
		`C:\Users\ada\Downloads`,
		`C:\Users\ada\Desktop\notes`,
		`C:\Users\ada\Documents\work`,
		`C:\Users\ada\OneDrive\doc`,
		`C:\Users\ada\AppData\Local`,
		`C:\Users\ada\AppData\Roaming\Microsoft\Windows`,
		`C:\Users\ada\AppData\Local\Google\Chrome\User Data`,
		`C:\Users\ada\AppData\Local\Google\Chrome\User Data\Default`,
		`C:\Users\ada\AppData\Local\Google\Chrome\User Data\Default\Local Storage`,
		`C:\Users\ada\AppData\Local\Google\Chrome\User Data\Default\Cookies`,
		`C:\Windows\SoftwareDistribution`,
		`C:\Windows\SoftwareDistribution\DataStore`,
		`D:\Windows\Temp`,
		`C:\Windows\Tempest`,
	}
	for _, path := range bad {
		if err := ValidateRoot(path, drive); err == nil {
			t.Errorf("ValidateRoot(%s) should fail", path)
		}
	}
}

func TestValidateDeleteRejectsEscape(t *testing.T) {
	root := `C:\Windows\Temp`
	escaped := `C:\Windows\Temp\..\System32\cmd.exe`
	if err := ValidateDelete(escaped, root, "C:", false); err == nil {
		t.Fatal("dotdot escape was allowed")
	}
	if err := ValidateDelete(`C:\Windows\Temp\cache\a.tmp`, root, "C:", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelete(root, root, "C:", true); err == nil {
		t.Fatal("deleting the root directory should be rejected")
	}
}

func TestWindowsOldCanBeDeletedWhenAllowed(t *testing.T) {
	root := `C:\Windows.old`
	if err := ValidateDelete(`C:\Windows.old\Users\a.txt`, root, "C:", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelete(root, root, "C:", true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelete(`C:\Windows\System32\cmd.exe`, root, "C:", false); err == nil {
		t.Fatal("system32 was allowed through Windows.old")
	}
}

func TestFilePolicies(t *testing.T) {
	if err := ValidateDelete(`C:\Windows\Logs\CBS\CBS.log`, `C:\Windows\Logs\CBS`, "C:", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelete(`C:\Windows\Logs\CBS\evil.exe`, `C:\Windows\Logs\CBS`, "C:", false); err == nil {
		t.Fatal("exe in the log folder was allowed")
	}
	if err := ValidateDelete(`C:\Users\ada\AppData\Local\Microsoft\Windows\Explorer\thumbcache_32.db`, `C:\Users\ada\AppData\Local\Microsoft\Windows\Explorer`, "C:", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelete(`C:\Users\ada\AppData\Local\Microsoft\Windows\Explorer\settings.dat`, `C:\Users\ada\AppData\Local\Microsoft\Windows\Explorer`, "C:", false); err == nil {
		t.Fatal("non-cache explorer file was allowed")
	}
	if err := ValidateDelete(`C:\Users\ada\AppData\Local\CrashDumps\app.dmp`, `C:\Users\ada\AppData\Local\CrashDumps`, "C:", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelete(`C:\Users\ada\AppData\Local\CrashDumps\notes.txt`, `C:\Users\ada\AppData\Local\CrashDumps`, "C:", false); err == nil {
		t.Fatal("txt in crash dumps was allowed")
	}
}

func TestAuthorizeRejectsReparseAndFinalPath(t *testing.T) {
	root := `C:\Windows\Temp`
	req := `C:\Windows\Temp\link`
	if err := Authorize(req, req, root, "C:", false, true); err == nil {
		t.Fatal("reparse point was allowed")
	}
	outside := `C:\Windows\System32\cmd.exe`
	if err := Authorize(req, outside, root, "C:", false, false); err == nil {
		t.Fatal("final path outside the root was allowed")
	}
	if err := Authorize(req, "", root, "C:", false, false); err == nil {
		t.Fatal("missing final path was allowed")
	}
	file := `C:\Windows\Temp\old.tmp`
	if err := Authorize(file, file, root, "C:", false, false); err != nil {
		t.Fatal(err)
	}
}

func TestProtectedFiles(t *testing.T) {
	if !ProtectedFile(`C:\pagefile.sys`) {
		t.Fatal("pagefile")
	}
	if err := ValidateDelete(`C:\Users\ada\AppData\Local\Temp\ntuser.dat`, `C:\Users\ada\AppData\Local\Temp`, "C:", false); err == nil {
		t.Fatal("registry hive inside temp was allowed")
	}
	if err := ValidateDelete(`C:\Windows.old\pagefile.sys`, `C:\Windows.old`, "C:", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelete(`C:\Windows\MEMORY.DMP`, `C:\Windows\MEMORY.DMP`, "C:", false); err != nil {
		t.Fatal(err)
	}
}
