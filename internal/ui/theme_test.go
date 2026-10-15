package ui

import (
	"testing"

	"cclear/internal/clean"
)

func TestThemesHaveDistinctTokens(t *testing.T) {
	light, dark := Light(), Dark()
	if light.Bg == dark.Bg || light.Text == light.Bg || dark.Text == dark.Bg {
		t.Fatal("theme tokens collide")
	}
	if Named("dark").Name != "dark" || Named("light").Name != "light" {
		t.Fatal("named theme")
	}
	if Radius != 8 || Space != 8 || TitlePx != 24 {
		t.Fatal("spacing tokens")
	}
}

func TestCleanSectionAndActionLabel(t *testing.T) {
	if CleanSection(cleanCat("user-temp", 1, true)) != SectionRecommend {
		t.Fatal("recommend")
	}
	if CleanSection(cleanCat("wer", 1, false)) != SectionOptional {
		t.Fatal("optional")
	}
	for _, id := range []string{"recycle", "dism", "winupdate", "winlogs", "crash"} {
		risk := 1
		if id == "crash" {
			risk = 2
		}
		if id == "dism" || id == "winupdate" {
			risk = 3
		}
		if CleanSection(cleanCat(id, risk, false)) != SectionAdvanced {
			t.Fatalf("%s not advanced", id)
		}
	}
	if CleanActionLabel(0, 0, false) != "请先选择清理项" {
		t.Fatal("empty")
	}
	if CleanActionLabel(1, 1024, false) != "清理 1 KB" {
		t.Fatal(CleanActionLabel(1, 1024, false))
	}
}

func cleanCat(id string, risk int, on bool) clean.Category {
	return clean.Category{ID: id, Risk: risk, DefaultOn: on}
}
