package sourcecheck

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUIHasNoCanvasHits(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "cmd", "cclear")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var banned = []string{
		"a.hits",
		"WM_DRAWITEM",
		"bsOwnerDraw",
		"hitAt(",
		"WM_LBUTTONDOWN",
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, item := range banned {
			if strings.Contains(text, item) {
				t.Fatalf("%s still contains %s", entry.Name(), item)
			}
		}
	}
}

func TestInstallerCloseUsesTimeout(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "cmd", "setup", "install_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "SendMessageTimeoutW") {
		t.Fatal("closeRunningApp must use SendMessageTimeoutW")
	}
	if strings.Contains(text, "SendMessageW") || strings.Contains(text, "TerminateProcess") || strings.Contains(text, "taskkill") {
		t.Fatal("installer must not block on SendMessageW or kill the process")
	}
	if !strings.Contains(text, "tx.Rollback()") || !strings.Contains(text, "itxn.Begin") {
		t.Fatal("install must stage through the rollback transaction")
	}
}

func TestReleaseWorkflowDoesNotPublish(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, need := range []string{"go test ./...", "go vet ./...", "GOARCH", "SHA256", "upload-artifact"} {
		if !strings.Contains(text, need) {
			t.Fatalf("workflow missing %s", need)
		}
	}
	for _, banned := range []string{"softprops/action-gh-release", "actions/create-release", "gh release create"} {
		if strings.Contains(text, banned) {
			t.Fatalf("workflow publishes a release via %s", banned)
		}
	}
	sign, err := os.ReadFile(filepath.Join(root, "scripts", "sign-windows.sh"))
	if err != nil {
		t.Fatal(err)
	}
	signed := string(sign)
	if !strings.Contains(signed, "https://github.com/leiz11904/CClear") {
		t.Fatal("sign URL")
	}
	if strings.Contains(signed, "example.invalid") {
		t.Fatal("example sign URL remains")
	}
	if !strings.Contains(signed, "CCLEAR_CERT") {
		t.Fatal("signing must stay behind CCLEAR_CERT")
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(readme)
	if !strings.Contains(doc, "至少保留最新一份") {
		t.Fatal("log retention copy")
	}
	if !strings.Contains(doc, "git 忽略") || !strings.Contains(doc, "GitHub Releases") {
		t.Fatal("distribution copy")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
