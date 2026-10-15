package clean

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverBrowserCachesSkipsProfilesAndPrivateData(t *testing.T) {
	local := t.TempDir()
	userData := filepath.Join(local, "Google", "Chrome", "User Data")
	cache := filepath.Join(userData, "Default", "Cache")
	localStorage := filepath.Join(userData, "Default", "Local Storage")
	cookies := filepath.Join(userData, "Default", "Cookies")
	codeCache := filepath.Join(userData, "Profile 2", "Code Cache")
	worker := filepath.Join(userData, "Profile 2", "Service Worker", "CacheStorage")
	for _, dir := range []string{cache, localStorage, codeCache, worker} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(cookies, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "Cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(userData, "Profile 1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(userData, "Default", "GPUCache")); err != nil {
		t.Fatal(err)
	}

	got := DiscoverBrowserCaches(local)
	if !containsFold(got, filepath.Join("Default", "Cache")) {
		t.Fatalf("missing default cache: %v", got)
	}
	if !containsFold(got, filepath.Join("Profile 2", "Code Cache")) {
		t.Fatalf("missing profile cache: %v", got)
	}
	if !containsFold(got, filepath.Join("CacheStorage")) {
		t.Fatalf("missing service worker cache: %v", got)
	}
	for _, blocked := range []string{"Local Storage", "Cookies", "GPUCache", outside} {
		if containsFold(got, blocked) {
			t.Fatalf("discovered %s in %#v", blocked, got)
		}
	}
}
