//go:build (production || dev) && windows

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"cclear/internal/clean"
	"cclear/internal/dupes"
)

func TestFeatureBindingsUseFrontendFieldNames(t *testing.T) {
	body, err := json.Marshal(CleanItem{ID: "temp", DefaultOn: true, NeedsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, field := range []string{`"id"`, `"defaultOn"`, `"needsAdmin"`} {
		if !strings.Contains(text, field) {
			t.Fatalf("missing %s in %s", field, text)
		}
	}
	if strings.Contains(text, `"ID"`) {
		t.Fatalf("Go field name leaked: %s", text)
	}
}

func TestDestructiveFeatureMethodsRequireExactConfirmation(t *testing.T) {
	a := &App{}
	if _, err := a.RunCleanup([]string{"temp"}, ""); err == nil {
		t.Fatal("cleanup accepted without confirmation")
	}
	if err := a.RecycleDuplicate(`C:\data\copy.bin`, ""); err == nil {
		t.Fatal("duplicate removal accepted without confirmation")
	}
	if err := a.LaunchUninstall("Example", ""); err == nil {
		t.Fatal("uninstall accepted without confirmation")
	}
	if err := a.RelaunchElevated(""); err == nil {
		t.Fatal("elevation accepted without confirmation")
	}
}

func TestCleanItemsBoundsPreviewAndPreservesSafetyMetadata(t *testing.T) {
	rep := clean.Report{Category: clean.Category{ID: "advanced", Name: "高级项目", Risk: 3, NeedsAdmin: true}, Bytes: 42}
	for i := 0; i < 80; i++ {
		rep.Files = append(rep.Files, clean.FileHit{Path: `C:\safe-root\candidate.tmp`})
	}
	rows := cleanItems([]clean.Report{rep})
	if len(rows) != 1 || len(rows[0].Preview) != 40 {
		t.Fatalf("unexpected preview size: %+v", rows)
	}
	if rows[0].Risk != 3 || !rows[0].NeedsAdmin || rows[0].ID != "advanced" {
		t.Fatalf("safety metadata lost: %+v", rows[0])
	}
}

func TestDuplicateResultSortsLargestGroupsFirst(t *testing.T) {
	rep := dupes.Report{Groups: []dupes.Group{{Hash: "small", Size: 10}, {Hash: "large", Size: 100}}}
	got := duplicateResult(rep)
	if len(got.Groups) != 2 || got.Groups[0].Hash != "large" {
		t.Fatalf("unexpected order: %+v", got.Groups)
	}
}
