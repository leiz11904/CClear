package ui

// Color is a Windows COLORREF stored as 0x00BBGGRR.
type Color uint32

func RGB(r, g, b byte) Color {
	return Color(r) | Color(g)<<8 | Color(b)<<16
}

// Theme is the shared visual token set. Win32 painting and tests both use it
// so pages do not hard-code colors.
type Theme struct {
	Name        string
	Bg          Color
	Card        Color
	Primary     Color
	PrimarySoft Color
	Text        Color
	Muted       Color
	Caption     Color
	Disabled    Color
	Line        Color
	Nav         Color
	NavPick     Color
	NavText     Color
	Danger      Color
	Warning     Color
	Success     Color
	Footer      Color
	AdvancedBg  Color
	RecommendBg Color
	Focus       Color
	OnPrimary   Color
	MapA, MapB  Color
	MapC, MapD  Color
}

func Light() Theme {
	return Theme{
		Name:        "light",
		Bg:          RGB(243, 245, 246),
		Card:        RGB(255, 255, 255),
		Primary:     RGB(42, 122, 130),
		PrimarySoft: RGB(231, 242, 243),
		Text:        RGB(22, 28, 30),
		Muted:       RGB(55, 67, 72),
		Caption:     RGB(70, 82, 88),
		Disabled:    RGB(132, 142, 146),
		Line:        RGB(226, 232, 234),
		Nav:         RGB(248, 250, 250),
		NavPick:     RGB(231, 242, 243),
		NavText:     RGB(27, 36, 38),
		Danger:      RGB(196, 43, 28),
		Warning:     RGB(196, 122, 18),
		Success:     RGB(15, 123, 58),
		Footer:      RGB(255, 255, 255),
		AdvancedBg:  RGB(255, 248, 240),
		RecommendBg: RGB(244, 250, 246),
		Focus:       RGB(42, 122, 130),
		OnPrimary:   RGB(255, 255, 255),
		MapA:        RGB(200, 224, 217),
		MapB:        RGB(210, 222, 232),
		MapC:        RGB(232, 220, 208),
		MapD:        RGB(216, 208, 228),
	}
}

func Dark() Theme {
	return Theme{
		Name:        "dark",
		Bg:          RGB(28, 31, 33),
		Card:        RGB(37, 41, 44),
		Primary:     RGB(74, 163, 171),
		PrimarySoft: RGB(30, 51, 54),
		Text:        RGB(242, 244, 245),
		Muted:       RGB(176, 186, 190),
		Caption:     RGB(154, 166, 171),
		Disabled:    RGB(110, 118, 122),
		Line:        RGB(52, 58, 62),
		Nav:         RGB(22, 25, 27),
		NavPick:     RGB(30, 51, 54),
		NavText:     RGB(242, 244, 245),
		Danger:      RGB(224, 90, 79),
		Warning:     RGB(224, 160, 74),
		Success:     RGB(76, 175, 122),
		Footer:      RGB(37, 41, 44),
		AdvancedBg:  RGB(58, 44, 30),
		RecommendBg: RGB(30, 46, 40),
		Focus:       RGB(74, 163, 171),
		OnPrimary:   RGB(16, 24, 26),
		MapA:        RGB(46, 72, 68),
		MapB:        RGB(48, 62, 78),
		MapC:        RGB(78, 62, 48),
		MapD:        RGB(62, 52, 78),
	}
}

func Named(name string) Theme {
	if name == "dark" {
		return Dark()
	}
	return Light()
}

const (
	Space      = 8
	Radius     = 8
	NavWidth   = 200
	FooterH    = 64
	ContentMax = 880
	TitlePx    = 24
	SectionPx  = 16
	BodyPx     = 14
	CaptionPx  = 13
)
