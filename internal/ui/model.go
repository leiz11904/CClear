package ui

import "cclear/internal/clean"

const (
	PageOverview = iota
	PageClean
	PageAnalyze
	PageDupes
	PageApps
	PageSystem
	PageHistory
	PageSettings
	PageRegular = PageClean
	PageDeep    = PageClean
)

type NavItem struct {
	Page  int
	Title string
	Hint  string
	Group string
	Icon  string
}

type NavGroup struct {
	Title string
	Items []NavItem
}

func NavItems() []NavItem {
	var out []NavItem
	for _, g := range NavGroups() {
		out = append(out, g.Items...)
	}
	out = append(out, SettingsItem())
	return out
}

func SettingsItem() NavItem {
	return NavItem{PageSettings, "设置", "外观、默认项与诊断", "settings", "settings"}
}

func NavGroups() []NavGroup {
	return []NavGroup{
		{"清理", []NavItem{
			{PageOverview, "总览", "磁盘与推荐操作", "clean", "home"},
			{PageClean, "垃圾清理", "查看结果并清理", "clean", "clean"},
			{PageHistory, "清理历史", "来自本机日志", "clean", "history"},
		}},
		{"工具", []NavItem{
			{PageAnalyze, "磁盘分析", "目录树与大文件", "tools", "disk"},
			{PageDupes, "重复文件", "只分析，不自动删除", "tools", "dupes"},
			{PageApps, "软件管理", "登记的卸载信息", "tools", "apps"},
			{PageSystem, "系统工具", "系统占用与快照", "tools", "system"},
		}},
	}
}

func GroupForPage(page int) string {
	if page == PageClean {
		return "clean"
	}
	return ""
}

func ReportsForPage(page int, reports []clean.Report) []clean.Report {
	if page == PageClean {
		return reports
	}
	return reports
}

func filterGroups(reports []clean.Report, groups ...string) []clean.Report {
	want := map[string]bool{}
	for _, g := range groups {
		want[g] = true
	}
	var out []clean.Report
	for _, rep := range reports {
		if want[rep.Category.Group] {
			out = append(out, rep)
		}
	}
	return out
}

type ScanHeadline struct {
	Ready   bool
	Title   string
	Detail  string
	Reclaim int64
	Count   int
	Unknown bool
	Items   int
}

func Headline(reports []clean.Report, scanned bool) ScanHeadline {
	if !scanned {
		return ScanHeadline{Title: "尚未扫描", Detail: "扫描系统盘上已核对的缓存和临时文件。文档、桌面和下载不会进入列表。"}
	}
	var h ScanHeadline
	h.Ready = true
	h.Title = "扫描完成"
	for _, rep := range reports {
		if rep.Unknown {
			h.Unknown = true
			h.Items++
			continue
		}
		if rep.Count > 0 || rep.Bytes > 0 {
			h.Items++
		}
		h.Count += rep.Count
		if rep.Bytes > 0 {
			h.Reclaim += rep.Bytes
		}
	}
	h.Detail = "预计可释放空间来自本次扫描，不是猜测。"
	return h
}

func SelectedSummary(reports []clean.Report, choices map[string]bool) (count int, reclaim int64, unknown bool, highRisk bool, warnings []string) {
	for _, rep := range reports {
		if choices == nil || !choices[rep.Category.ID] {
			continue
		}
		count++
		if CleanSection(rep.Category) == SectionAdvanced {
			highRisk = true
		}
		if rep.Category.Warning != "" {
			warnings = append(warnings, rep.Category.Name+"："+rep.Category.Warning)
		}
		if rep.Unknown {
			unknown = true
			continue
		}
		if rep.Bytes > 0 {
			reclaim += rep.Bytes
		}
	}
	return
}

func SizeText(rep clean.Report) string {
	if rep.Unknown {
		return "待系统分析"
	}
	if len(rep.Errors) > 0 && rep.Bytes == 0 {
		return "无法访问"
	}
	return clean.FormatBytes(rep.Bytes)
}

func RiskText(risk int) string {
	switch risk {
	case 3:
		return "高风险"
	case 2:
		return "中风险"
	default:
		return "低风险"
	}
}

func ConfirmExtra(reports []clean.Report, choices map[string]bool) []string {
	var out []string
	for _, rep := range reports {
		if choices == nil || !choices[rep.Category.ID] {
			continue
		}
		if CleanSection(rep.Category) != SectionAdvanced {
			continue
		}
		msg := rep.Category.Name + "：" + RiskReason(rep.Category) + " " + Recoverable(rep.Category)
		out = append(out, msg)
	}
	return out
}
