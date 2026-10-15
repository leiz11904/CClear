package prefs

import "testing"

func TestSaveLoadChoices(t *testing.T) {
	base := t.TempDir()
	if err := Save(base, File{Choices: map[string]bool{"user-temp": true, "dism": false}}); err != nil {
		t.Fatal(err)
	}
	got := Load(base)
	if !got.Choices["user-temp"] || got.Choices["dism"] {
		t.Fatalf("%v", got.Choices)
	}
}

func TestMissingFile(t *testing.T) {
	got := Load(t.TempDir())
	if got.Choices == nil {
		t.Fatal("nil map")
	}
}

func TestThemeAndLogKeep(t *testing.T) {
	base := t.TempDir()
	if err := Save(base, File{Theme: "dark", LogKeepMB: 5}); err != nil {
		t.Fatal(err)
	}
	got := Load(base)
	if got.Theme != "dark" || got.LogKeepMB != 5 {
		t.Fatalf("%+v", got)
	}
	if ClampLogKeepMB(9) != 2 {
		t.Fatal("default log keep")
	}
}
