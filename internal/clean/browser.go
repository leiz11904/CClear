package clean

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var profileName = regexp.MustCompile(`^(Default|Guest Profile|Profile \d+)$`)

var chromiumCacheDirs = []string{
	"Cache",
	"Code Cache",
	"GPUCache",
	"DawnGraphiteCache",
	"DawnWebGPUCache",
	"ShaderCache",
	"GrShaderCache",
}

// DiscoverBrowserCaches returns cache directories for installed browsers.
// Bookmarks, passwords, cookies, and site storage are not included.
func DiscoverBrowserCaches(localAppData string) []string {
	var roots []string
	for _, family := range []string{"chrome", "edge", "firefox", "other"} {
		roots = append(roots, DiscoverFamily(localAppData, family)...)
	}
	return roots
}

// DiscoverFamily returns cache directories for one browser family.
func DiscoverFamily(localAppData, family string) []string {
	if localAppData == "" {
		return nil
	}
	switch family {
	case "chrome":
		return chromiumFamily(localAppData, []string{
			filepath.Join("Google", "Chrome"),
			filepath.Join("Google", "Chrome Beta"),
			filepath.Join("Google", "Chrome SxS"),
		})
	case "edge":
		return chromiumFamily(localAppData, []string{
			filepath.Join("Microsoft", "Edge"),
			filepath.Join("Microsoft", "Edge Beta"),
			filepath.Join("Microsoft", "Edge Dev"),
		})
	case "firefox":
		return firefoxCaches(filepath.Join(localAppData, "Mozilla", "Firefox", "Profiles"))
	case "other":
		roots := chromiumFamily(localAppData, []string{
			filepath.Join("BraveSoftware", "Brave-Browser"),
			filepath.Join("Chromium"),
		})
		opera := filepath.Join(localAppData, "Opera Software", "Opera Stable")
		for _, name := range []string{"Cache", "GPUCache", "Code Cache"} {
			if dir, ok := realDir(filepath.Join(opera, name)); ok {
				roots = append(roots, dir)
			}
		}
		return roots
	default:
		return nil
	}
}

func chromiumFamily(localAppData string, browsers []string) []string {
	var roots []string
	for _, browser := range browsers {
		roots = append(roots, chromiumProfileCaches(filepath.Join(localAppData, browser, "User Data"))...)
	}
	return roots
}

func chromiumProfileCaches(userData string) []string {
	entries, err := os.ReadDir(userData)
	if err != nil {
		return nil
	}
	var roots []string
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !profileName.MatchString(entry.Name()) {
			continue
		}
		profile := filepath.Join(userData, entry.Name())
		if !realDirPath(profile) {
			continue
		}
		for _, cache := range chromiumCacheDirs {
			if dir, ok := realDir(filepath.Join(profile, cache)); ok {
				roots = append(roots, dir)
			}
		}
		if dir, ok := realDir(filepath.Join(profile, "Service Worker", "CacheStorage")); ok {
			roots = append(roots, dir)
		}
	}
	return roots
}

func firefoxCaches(profiles string) []string {
	entries, err := os.ReadDir(profiles)
	if err != nil {
		return nil
	}
	var roots []string
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		profile := filepath.Join(profiles, entry.Name())
		if !realDirPath(profile) {
			continue
		}
		for _, name := range []string{"cache2", "startupCache", "shader-cache", "thumbnails"} {
			if dir, ok := realDir(filepath.Join(profile, name)); ok {
				roots = append(roots, dir)
			}
		}
	}
	return roots
}

func realDir(path string) (string, bool) {
	if realDirPath(path) {
		return path, true
	}
	return "", false
}

func realDirPath(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func containsFold(list []string, part string) bool {
	for _, item := range list {
		if strings.Contains(strings.ToLower(item), strings.ToLower(part)) {
			return true
		}
	}
	return false
}
