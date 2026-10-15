package clean

import (
	"strings"
	"testing"

	"cclear/internal/rules"
	"cclear/internal/winpath"
)

func TestSpaceFeaturesDidNotChangeCleanupRules(t *testing.T) {
	list, err := rules.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 18 {
		t.Fatalf("rules %d", len(list))
	}
	for _, rule := range list {
		joined := strings.ToLower(strings.Join(rule.Tokens, " "))
		if strings.Contains(joined, "document") || strings.Contains(joined, "desktop") || strings.Contains(joined, "download") {
			t.Fatalf("%s token mentions user data: %v", rule.ID, rule.Tokens)
		}
	}
	protected := []string{
		`C:\Users\ada\Documents\file.txt`,
		`C:\Users\ada\Desktop\note.txt`,
		`C:\Users\ada\Downloads\setup.exe`,
	}
	for _, path := range protected {
		cleaned, err := winpath.Clean(path)
		if err != nil {
			t.Fatal(err)
		}
		if !winpath.ProtectedUserData(cleaned) {
			t.Fatalf("not protected: %s", cleaned)
		}
	}
}

func TestReclaimableStillPrefersAllocated(t *testing.T) {
	if Reclaimable(100, 40, 1) != 40 {
		t.Fatal("logical size was used as disk use")
	}
}
