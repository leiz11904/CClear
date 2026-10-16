package ui

import "testing"

func TestFooterQuietOnSettingsAndHistory(t *testing.T) {
	if FooterMode(PageSettings) != FooterQuiet || FooterMode(PageHistory) != FooterQuiet {
		t.Fatal("settings and history must not use the cleanup footer")
	}
	if FooterHasCleanActions(PageSettings) || FooterHasCleanActions(PageHistory) {
		t.Fatal("quiet pages must not expose scan or clean actions")
	}
	if FooterHasCleanActions(PageAnalyze) || FooterHasCleanActions(PageDupes) || FooterHasCleanActions(PageApps) || FooterHasCleanActions(PageSystem) {
		t.Fatal("tool pages use their own action, not cleanup")
	}
	if !FooterHasCleanActions(PageOverview) || !FooterHasCleanActions(PageClean) {
		t.Fatal("overview and clean keep their own footer actions")
	}
	if FooterMode(PageAnalyze) != FooterAnalyze || FooterMode(PageLarge) != FooterAnalyze || FooterMode(PageSystem) != FooterSystem {
		t.Fatal("tool footer modes")
	}
	if FooterHasCleanActions(PageLarge) {
		t.Fatal("large files are not a cleanup page")
	}
}

func TestAccessNamesCarryState(t *testing.T) {
	if NavAccessName("垃圾清理", true) != "垃圾清理，当前页" {
		t.Fatal(NavAccessName("垃圾清理", true))
	}
	if NavAccessName("垃圾清理", false) != "垃圾清理" {
		t.Fatal("plain nav name")
	}
	if AdvancedWarning() != "默认关闭，需手动确认" {
		t.Fatal(AdvancedWarning())
	}
	if SectionHint(SectionAdvanced) == "" || !containsAll(SectionHint(SectionAdvanced), AdvancedWarning()) {
		t.Fatal(SectionHint(SectionAdvanced))
	}
	note := AdvancedItemNote("缓存可重建。")
	if !containsAll(note, AdvancedWarning(), "缓存可重建。") {
		t.Fatal(note)
	}
	choice := ChoiceAccessName("组件存储", "高风险", true)
	if !containsAll(choice, "组件存储", "高风险", AdvancedWarning()) {
		t.Fatal(choice)
	}
	if DetailAccessName("临时文件", false) != "查看临时文件的详情" {
		t.Fatal(DetailAccessName("临时文件", false))
	}
	if ThemeAccessName(true, true) != "深色主题，已选择" {
		t.Fatal(ThemeAccessName(true, true))
	}
	keep := LogKeepAccessName(2, true)
	if !containsAll(keep, "2 MB", "至少保留最新一份", "已选择") {
		t.Fatal(keep)
	}
	if !containsAll(LogKeepHint(), "至少保留最新一份") {
		t.Fatal(LogKeepHint())
	}
	if HistoryOpenName("2026-10-14 15:04") != "打开 2026-10-14 15:04 的日志" {
		t.Fatal(HistoryOpenName("2026-10-14 15:04"))
	}
	if FileActionName("recycle", "a.bin") != "将 a.bin 移到回收站" {
		t.Fatal(FileActionName("recycle", "a.bin"))
	}
	if TreeAccessName("Windows", true, false) != "展开目录 Windows" {
		t.Fatal(TreeAccessName("Windows", true, false))
	}
	if MapAccessName("Users") != "选择目录 Users" {
		t.Fatal(MapAccessName("Users"))
	}
	names := []string{
		NavAccessName("总览", false),
		AdvancedToggleName(false),
		ResultToggleName(false),
		FilterAccessName("大于 100MB", true),
	}
	for _, name := range names {
		if name == "" {
			t.Fatal("empty access name")
		}
	}
}

func TestSummaryColumnsDoNotOverlap(t *testing.T) {
	for _, inner := range []int{480, 640, 712, 880, 1200} {
		left, width := SummaryColumns(inner)
		prev := 0
		for i := 0; i < 4; i++ {
			if width[i] <= 0 {
				t.Fatalf("inner %d width %v", inner, width)
			}
			if left[i] < prev {
				t.Fatalf("inner %d overlap %v %v", inner, left, width)
			}
			edge := left[i] + width[i]
			if i < 3 && edge > left[i+1] {
				t.Fatalf("inner %d collision %v %v", inner, left, width)
			}
			if edge > inner {
				t.Fatalf("inner %d overflow %d", inner, edge)
			}
			prev = edge
		}
		if inner >= 640 && width[0] < DateColumnW {
			t.Fatalf("date column %d at %d", width[0], inner)
		}
	}
}

func TestReclaimAndPendingUseDifferentColumns(t *testing.T) {
	left, width := SummaryColumns(712)
	if left[1] == left[2] || width[1] <= 0 || width[2] <= 0 {
		t.Fatalf("columns %v %v", left, width)
	}
	if left[1]+width[1] > left[2] {
		t.Fatal("reclaim and pending share a column")
	}
}

func TestEmphasisUsesRealBold(t *testing.T) {
	for role := TypePageTitle; role <= TypePath; role++ {
		w := TypeSpecOf(role).Weight
		if w != WeightRegular && w != 700 {
			t.Fatalf("role %d weight %d", role, w)
		}
	}
	if TypeSpecOf(TypeBody).Weight != 400 {
		t.Fatal("body")
	}
	if TypeSpecOf(TypePageTitle).Weight != 700 || TypeSpecOf(TypeButton).Weight != 700 || TypeSpecOf(TypeItem).Weight != 700 || TypeSpecOf(TypeNav).Weight != 700 {
		t.Fatal("emphasis must be bold 700")
	}
}

func TestScrollTailAndInset(t *testing.T) {
	if ScrollTail < 24 || TextInset < 1 || ScrollBarW < 12 {
		t.Fatal("spacing")
	}
}

func TestDarkControlsStayReadable(t *testing.T) {
	for _, th := range []Theme{Light(), Dark()} {
		pairs := [][2]Color{
			{th.Text, th.Bg},
			{th.Text, th.Card},
			{th.Text, th.Nav},
			{th.Text, th.NavPick},
			{th.Text, th.Footer},
			{th.Text, th.AdvancedBg},
			{th.Text, th.RecommendBg},
			{th.OnPrimary, th.Primary},
			{th.Caption, th.Card},
		}
		for _, pair := range pairs {
			if ContrastRatio(pair[0], pair[1]) < 4.5 {
				t.Fatalf("%s contrast %f", th.Name, ContrastRatio(pair[0], pair[1]))
			}
		}
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if part == "" || !contains(s, part) {
			return false
		}
	}
	return true
}

func contains(s, part string) bool {
	return len(s) >= len(part) && (s == part || len(part) == 0 || (len(s) > 0 && indexOf(s, part) >= 0))
}

func indexOf(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
