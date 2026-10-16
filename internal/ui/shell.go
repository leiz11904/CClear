package ui

import "fmt"

const (
	FooterQuiet = iota
	FooterOverview
	FooterClean
	FooterAnalyze
	FooterDupes
	FooterApps
	FooterSystem
)

const (
	ScrollTail  = 32
	ScrollBarW  = 17
	TextInset   = 2
	DateColumnW = 168
)

// FooterMode says which bottom actions a page may show.
// Settings and history stay quiet so cleanup actions cannot appear there.
func FooterMode(page int) int {
	switch page {
	case PageOverview:
		return FooterOverview
	case PageClean:
		return FooterClean
	case PageAnalyze, PageLarge:
		return FooterAnalyze
	case PageDupes:
		return FooterDupes
	case PageApps:
		return FooterApps
	case PageSystem:
		return FooterSystem
	default:
		return FooterQuiet
	}
}

func FooterHasCleanActions(page int) bool {
	switch FooterMode(page) {
	case FooterOverview, FooterClean:
		return true
	default:
		return false
	}
}

func AdvancedWarning() string {
	return "默认关闭，需手动确认"
}

func AdvancedItemNote(recoverable string) string {
	note := AdvancedWarning() + "。"
	if recoverable != "" {
		note += recoverable
	}
	return note
}

func LogKeepHint() string {
	return "超过上限时删除最旧的日志，至少保留最新一份。不会改写日志正文里的历史版本号。"
}

func NavAccessName(title string, current bool) string {
	if title == "" {
		title = "页面"
	}
	if current {
		return title + "，当前页"
	}
	return title
}

func AdvancedToggleName(open bool) string {
	if open {
		return "折叠高级清理"
	}
	return "展开高级清理"
}

func ResultToggleName(open bool) string {
	if open {
		return "收起清理详细记录"
	}
	return "查看清理详细记录"
}

func ChoiceAccessName(name, risk string, advanced bool) string {
	if name == "" {
		name = "清理项"
	}
	if risk == "" {
		risk = "未分级"
	}
	s := name + "，" + risk
	if advanced {
		s += "，" + AdvancedWarning()
	}
	return s
}

func DetailAccessName(name string, expanded bool) string {
	if name == "" {
		name = "此项"
	}
	if expanded {
		return "收起" + name + "的详情"
	}
	return "查看" + name + "的详情"
}

func ThemeAccessName(dark, selected bool) string {
	name := "浅色主题"
	if dark {
		name = "深色主题"
	}
	if selected {
		name += "，已选择"
	}
	return name
}

func LogKeepAccessName(mb int, selected bool) string {
	name := fmt.Sprintf("日志保留 %d MB，至少保留最新一份", mb)
	if selected {
		name += "，已选择"
	}
	return name
}

func HistoryOpenName(when string) string {
	if when == "" {
		return "打开这条清理日志"
	}
	return "打开 " + when + " 的日志"
}

func FileActionName(action, name string) string {
	if name == "" {
		name = "该文件"
	}
	switch action {
	case "open":
		return "查看 " + name
	case "reveal":
		return "打开 " + name + " 所在位置"
	case "copy":
		return "复制 " + name + " 的路径"
	case "props":
		return "查看 " + name + " 的属性"
	case "recycle":
		return "将 " + name + " 移到回收站"
	default:
		return action + " " + name
	}
}

func TreeAccessName(name string, expandable, open bool) string {
	if name == "" {
		name = "目录"
	}
	if !expandable {
		return "选择目录 " + name
	}
	if open {
		return "折叠目录 " + name
	}
	return "展开目录 " + name
}

func MapAccessName(name string) string {
	if name == "" {
		return "选择此目录"
	}
	return "选择目录 " + name
}

func FilterAccessName(label string, selected bool) string {
	name := "筛选 " + label
	if selected {
		name += "，已选择"
	}
	return name
}

// SummaryColumns splits a summary strip into four non-overlapping columns.
// Column 0 is wide enough for a yyyy-mm-dd hh:mm value when the strip allows it.
func SummaryColumns(inner int) (left, width [4]int) {
	gap := 12
	if inner < gap*3+4 {
		gap = 0
	}
	usable := inner - gap*3
	if usable < 4 {
		usable = inner
		if usable < 4 {
			usable = 4
		}
	}
	date := DateColumnW
	minOther := 72
	if usable < date+minOther*3 {
		date = usable / 4
		if date < 1 {
			date = 1
		}
	}
	rest := usable - date
	if rest < 3 {
		rest = 3
		if date > usable-rest {
			date = usable - rest
		}
	}
	width[0] = date
	width[1] = rest / 3
	width[2] = rest / 3
	width[3] = rest - width[1] - width[2]
	x := 0
	for i := 0; i < 4; i++ {
		left[i] = x
		x += width[i]
		if i < 3 {
			x += gap
		}
	}
	return left, width
}
