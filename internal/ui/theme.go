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
		Bg:          RGB(243, 243, 243),
		Card:        RGB(255, 255, 255),
		Primary:     RGB(0, 95, 184),
		PrimarySoft: RGB(232, 242, 251),
		Text:        RGB(27, 27, 27),
		Muted:       RGB(82, 82, 82),
		Caption:     RGB(96, 96, 96),
		Disabled:    RGB(140, 140, 140),
		Line:        RGB(224, 224, 224),
		Nav:         RGB(249, 249, 249),
		NavPick:     RGB(232, 242, 251),
		NavText:     RGB(27, 27, 27),
		Danger:      RGB(196, 43, 28),
		Warning:     RGB(157, 93, 0),
		Success:     RGB(15, 123, 58),
		Footer:      RGB(255, 255, 255),
		AdvancedBg:  RGB(255, 248, 240),
		RecommendBg: RGB(243, 248, 244),
		Focus:       RGB(0, 95, 184),
		OnPrimary:   RGB(255, 255, 255),
		MapA:        RGB(204, 224, 242),
		MapB:        RGB(214, 228, 216),
		MapC:        RGB(236, 224, 208),
		MapD:        RGB(220, 214, 232),
	}
}

func Dark() Theme {
	return Theme{
		Name:        "dark",
		Bg:          RGB(32, 32, 32),
		Card:        RGB(43, 43, 43),
		Primary:     RGB(96, 205, 255),
		PrimarySoft: RGB(28, 52, 68),
		Text:        RGB(255, 255, 255),
		Muted:       RGB(200, 200, 200),
		Caption:     RGB(196, 196, 196),
		Disabled:    RGB(120, 120, 120),
		Line:        RGB(64, 64, 64),
		Nav:         RGB(28, 28, 28),
		NavPick:     RGB(48, 48, 48),
		NavText:     RGB(255, 255, 255),
		Danger:      RGB(255, 153, 164),
		Warning:     RGB(252, 225, 0),
		Success:     RGB(108, 203, 95),
		Footer:      RGB(43, 43, 43),
		AdvancedBg:  RGB(58, 46, 32),
		RecommendBg: RGB(32, 48, 38),
		Focus:       RGB(96, 205, 255),
		OnPrimary:   RGB(16, 16, 16),
		MapA:        RGB(36, 64, 86),
		MapB:        RGB(36, 62, 48),
		MapC:        RGB(72, 56, 40),
		MapD:        RGB(56, 48, 72),
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
	NavWidth   = 216
	FooterH    = 56
	ContentMax = 880
	TitlePx    = 24
	SectionPx  = 16
	BodyPx     = 14
	CaptionPx  = 13
)
