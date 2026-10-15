package ui

import (
	"testing"
	"time"

	"cclear/internal/clean"
)

func TestReportsForPage(t *testing.T) {
	reports := []clean.Report{
		{Category: clean.Category{ID: "a", Group: "regular"}},
		{Category: clean.Category{ID: "b", Group: "recycle"}},
		{Category: clean.Category{ID: "c", Group: "deep"}},
		{Category: clean.Category{ID: "d", Group: "system"}},
	}
	if len(ReportsForPage(PageClean, reports)) != 4 {
		t.Fatal("clean page should show every cleanup group")
	}
	items := NavItems()
	if len(items) != 8 {
		t.Fatalf("nav %d", len(items))
	}
	if items[len(items)-1].Page != PageSettings {
		t.Fatal("settings must be last")
	}
	if items[6].Title != "系统工具" {
		t.Fatalf("system title %s", items[6].Title)
	}
	if len(NavGroups()) != 2 || NavGroups()[0].Title != "清理" || NavGroups()[1].Title != "工具" {
		t.Fatal("nav groups")
	}
}

func TestHeadlineUsesScanData(t *testing.T) {
	if Headline(nil, false).Ready {
		t.Fatal("empty state should not look scanned")
	}
	h := Headline([]clean.Report{
		{Bytes: 1024, Count: 2, Category: clean.Category{ID: "user-temp"}},
		{Unknown: true, Category: clean.Category{ID: "dism"}},
	}, true)
	if !h.Ready || h.Reclaim != 1024 || !h.Unknown || h.Items != 2 {
		t.Fatalf("%+v", h)
	}
}

func TestConfirmExtraOnlyForHighRisk(t *testing.T) {
	reports := []clean.Report{
		{Category: clean.Category{ID: "user-temp", Risk: 1}},
		{Category: clean.Category{ID: "windowsold", Risk: 3, Warning: "无法回退"}},
		{Category: clean.Category{ID: "recycle", Risk: 1}},
	}
	choices := map[string]bool{"user-temp": true, "windowsold": true, "recycle": true}
	got := ConfirmExtra(reports, choices)
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func TestSizeTextUnknown(t *testing.T) {
	if SizeText(clean.Report{Unknown: true}) != "待系统分析" {
		t.Fatal(SizeText(clean.Report{Unknown: true}))
	}
}

func TestReclaimAndPendingStaySeparate(t *testing.T) {
	h := Headline([]clean.Report{
		{Bytes: 4 << 30, Count: 1, Category: clean.Category{ID: "user-temp"}},
		{Unknown: true, Category: clean.Category{ID: "dism"}},
	}, true)
	if ReclaimValue(h) != "4 GB" {
		t.Fatalf("reclaim %s", ReclaimValue(h))
	}
	if PendingAnalysis(h) != "有项目待系统分析" {
		t.Fatalf("pending %s", PendingAnalysis(h))
	}
	if FormatScanAt(time.Date(2026, 10, 14, 15, 4, 0, 0, time.Local)) != "2026-10-14 15:04" {
		t.Fatal(FormatScanAt(time.Date(2026, 10, 14, 15, 4, 0, 0, time.Local)))
	}
	if HighRiskValue(5) != "5 项" || HighRiskExplain() == "" {
		t.Fatal("high risk copy")
	}
	if CleanCardH(false, false) > 60 {
		t.Fatalf("card too tall %d", CleanCardH(false, false))
	}
}
