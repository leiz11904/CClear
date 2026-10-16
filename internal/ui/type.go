package ui

// TypeRole is a reusable text style. Pages select a role; they do not pick
// font family, pixel size, or weight on their own.
type TypeRole int

const (
	TypePageTitle TypeRole = iota
	TypePageHint
	TypeSection
	TypeItem
	TypeBody
	TypeCaption
	TypeButton
	TypeBadge
	TypeMetric
	TypeNav
	TypeDialogTitle
	TypePath
	typeRoleCount
)

// TypeSpec is the device-independent definition of a text role.
// Px is CSS-style pixels at 96 DPI. Weight is a LOGFONT lfWeight.
type TypeSpec struct {
	Px     int
	Weight int
	Line   float64
	Mono   bool
}

const (
	WeightRegular = 400
	// YaHei UI has Regular and Bold. 500/600 are synthesized and blur CJK.
	WeightMedium   = 700
	WeightSemibold = 700
)

func TypeSpecOf(role TypeRole) TypeSpec {
	switch role {
	case TypePageTitle:
		return TypeSpec{Px: 20, Weight: WeightSemibold, Line: 1.3}
	case TypePageHint:
		return TypeSpec{Px: 14, Weight: WeightRegular, Line: 1.5}
	case TypeSection:
		return TypeSpec{Px: 16, Weight: WeightSemibold, Line: 1.35}
	case TypeItem:
		return TypeSpec{Px: 14, Weight: WeightMedium, Line: 1.4}
	case TypeBody:
		return TypeSpec{Px: 14, Weight: WeightRegular, Line: 1.5}
	case TypeCaption:
		return TypeSpec{Px: 13, Weight: WeightRegular, Line: 1.45}
	case TypeButton:
		return TypeSpec{Px: 14, Weight: WeightMedium, Line: 1.2}
	case TypeBadge:
		return TypeSpec{Px: 12, Weight: WeightMedium, Line: 1.2}
	case TypeMetric:
		return TypeSpec{Px: 22, Weight: WeightSemibold, Line: 1.2}
	case TypeNav:
		return TypeSpec{Px: 14, Weight: WeightMedium, Line: 1.2}
	case TypeDialogTitle:
		return TypeSpec{Px: 18, Weight: WeightSemibold, Line: 1.3}
	case TypePath:
		return TypeSpec{Px: 13, Weight: WeightRegular, Line: 1.45, Mono: true}
	default:
		return TypeSpec{Px: 14, Weight: WeightRegular, Line: 1.5}
	}
}

func LinePx(role TypeRole) int {
	spec := TypeSpecOf(role)
	h := int(float64(spec.Px)*spec.Line + 0.5)
	if h < spec.Px+4 {
		h = spec.Px + 4
	}
	return h
}

func ItemNameToCaptionGap() int { return 5 }

// FontStack is the UI face list. CJK and Latin share one family so mixed
// strings keep a common baseline. Microsoft YaHei UI is preferred; later
// faces are only used when earlier ones are missing.
func FontStack() []string {
	return []string{
		"Microsoft YaHei UI",
		"Microsoft YaHei",
		"Segoe UI Variable",
		"Segoe UI",
	}
}

func PathFontStack() []string {
	return []string{
		"Cascadia Mono",
		"Consolas",
		"Microsoft YaHei UI",
		"Segoe UI",
	}
}

func FormatDrive(letter string) string {
	if letter == "" {
		return "系统盘"
	}
	return string([]rune(letter)[0]) + " 盘"
}

func FormatVersion(version string) string {
	return "版本 " + version
}

func ContrastRatio(fg, bg Color) float64 {
	L1 := relativeLuminance(fg)
	L2 := relativeLuminance(bg)
	if L1 < L2 {
		L1, L2 = L2, L1
	}
	return (L1 + 0.05) / (L2 + 0.05)
}

func relativeLuminance(c Color) float64 {
	r := linearChannel(byte(c))
	g := linearChannel(byte(c >> 8))
	b := linearChannel(byte(c >> 16))
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func linearChannel(v byte) float64 {
	s := float64(v) / 255
	if s <= 0.04045 {
		return s / 12.92
	}
	x := (s + 0.055) / 1.055
	return x * x * x
}
