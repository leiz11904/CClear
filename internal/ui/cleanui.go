package ui

import (
	"fmt"
	"time"

	"cclear/internal/clean"
)

const (
	SectionRecommend = 1
	SectionOptional  = 2
	SectionAdvanced  = 3
)

// CleanSection is a UI grouping only. It does not change scan roots or delete rules.
func CleanSection(cat clean.Category) int {
	if cat.Risk >= 2 || cat.ID == "recycle" || cat.ID == "winlogs" || cat.ID == "winupdate" || cat.ID == "dism" || cat.ID == "windowsold" {
		return SectionAdvanced
	}
	if cat.DefaultOn {
		return SectionRecommend
	}
	return SectionOptional
}

func SectionTitle(section int) string {
	switch section {
	case SectionRecommend:
		return "推荐清理"
	case SectionOptional:
		return "可选清理"
	case SectionAdvanced:
		return "高级清理"
	default:
		return ""
	}
}

func SectionHint(section int) string {
	switch section {
	case SectionRecommend:
		return "风险较低的临时文件和缓存，默认选中。"
	case SectionOptional:
		return "可用，但默认不选。请先阅读说明。"
	case SectionAdvanced:
		return "可能影响系统还原、更新或排障。默认关闭，需展开后手动勾选。"
	default:
		return ""
	}
}

func ReportsInSection(reports []clean.Report, section int) []clean.Report {
	var out []clean.Report
	for _, rep := range reports {
		if CleanSection(rep.Category) == section {
			out = append(out, rep)
		}
	}
	return out
}

func IndexInReports(reports []clean.Report, id string) int {
	for i, rep := range reports {
		if rep.Category.ID == id {
			return i
		}
	}
	return -1
}

func CountHighRisk(reports []clean.Report) int {
	n := 0
	for _, rep := range reports {
		if CleanSection(rep.Category) == SectionAdvanced && (rep.Bytes > 0 || rep.Unknown || rep.Count > 0) {
			n++
		}
	}
	return n
}

func CountCategories(reports []clean.Report) int {
	n := 0
	for _, rep := range reports {
		if rep.Unknown || rep.Count > 0 || rep.Bytes > 0 {
			n++
		}
	}
	return n
}

func Recoverable(cat clean.Category) string {
	switch cat.ID {
	case "recycle":
		return "不可恢复：清空后不能从回收站还原。"
	case "windowsold":
		return "不可恢复：删除后无法回退到升级前的 Windows。"
	case "dism":
		return "由 Windows 执行组件清理，本程序不能还原已取代的组件。"
	case "winupdate":
		return "已下载的更新包删除后需要重新获取，不会卸载已安装的更新。"
	case "winlogs":
		return "删除后，已有更新日志不能再用于排查失败原因。"
	}
	if cat.Risk >= 3 {
		return "系统级操作，删除后通常不能由本程序还原。"
	}
	if cat.Risk == 2 {
		return "缓存可重建，但当前文件删除后不能还原。"
	}
	return "删除后不进回收站。应用之后可以重新生成这些临时文件。"
}

func RiskReason(cat clean.Category) string {
	if cat.Warning != "" {
		return cat.Warning
	}
	return Recoverable(cat)
}

func CleanActionLabel(count int, reclaim int64, unknown bool) string {
	if count == 0 {
		return "请先选择清理项"
	}
	if reclaim > 0 {
		return "清理 " + clean.FormatBytes(reclaim)
	}
	if unknown {
		return "清理所选"
	}
	return "清理所选"
}

func LastCleanFreed(historyEstimated string, diskDelta int64) string {
	if diskDelta != 0 {
		return clean.FormatBytes(diskDelta)
	}
	if historyEstimated != "" {
		return historyEstimated
	}
	return "尚无清理记录"
}

func CountUnknown(reports []clean.Report) int {
	n := 0
	for _, rep := range reports {
		if rep.Unknown {
			n++
		}
	}
	return n
}

func ReclaimValue(head ScanHeadline) string {
	if !head.Ready {
		return "尚未扫描"
	}
	return clean.FormatBytes(head.Reclaim)
}

func PendingAnalysis(head ScanHeadline) string {
	if !head.Ready {
		return "—"
	}
	if !head.Unknown {
		return "无"
	}
	return "有项目待系统分析"
}

func HighRiskValue(n int) string {
	if n <= 0 {
		return "0 项"
	}
	return fmt.Sprintf("%d 项", n)
}

func HighRiskExplain() string {
	return "默认关闭，需手动确认"
}

func FormatScanAt(t time.Time) string {
	if t.IsZero() {
		return "尚未扫描"
	}
	return t.Format("2006-01-02 15:04")
}

func CleanCardH(advanced, expanded bool) int {
	h := 56
	if advanced {
		h += 18
	}
	if expanded {
		h += 84
	}
	return h
}
