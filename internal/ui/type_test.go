package ui

import "testing"

func TestTypeScale(t *testing.T) {
	if TypeSpecOf(TypePageTitle).Px != 20 || TypeSpecOf(TypePageTitle).Weight != WeightSemibold {
		t.Fatal("page title")
	}
	if TypeSpecOf(TypeBody).Px != 14 || TypeSpecOf(TypeBody).Weight != WeightRegular {
		t.Fatal("body must stay Regular for Chinese")
	}
	if TypeSpecOf(TypeItem).Weight != 700 || TypeSpecOf(TypeButton).Weight != 700 {
		t.Fatal("CJK emphasis must use real Bold, not synthesized 500")
	}
	if TypeSpecOf(TypePath).Mono == false {
		t.Fatal("paths are mono")
	}
	if LinePx(TypeBody) < 19 || LinePx(TypeBody) > 24 {
		t.Fatalf("body line %d", LinePx(TypeBody))
	}
	if ItemNameToCaptionGap() < 4 || ItemNameToCaptionGap() > 6 {
		t.Fatal("item gap")
	}
}

func TestFontStackOrder(t *testing.T) {
	stack := FontStack()
	if stack[0] != "Microsoft YaHei UI" || stack[1] != "Microsoft YaHei" {
		t.Fatalf("%v", stack)
	}
	for _, face := range stack {
		if face == "SimSun" || face == "Arial" {
			t.Fatal(face)
		}
	}
}

func TestFormatDriveAndVersion(t *testing.T) {
	if FormatDrive("C") != "C 盘" || FormatDrive("C:") != "C 盘" {
		t.Fatal(FormatDrive("C"))
	}
	if FormatVersion("2.7.0") != "版本 2.7.0" {
		t.Fatal(FormatVersion("2.7.0"))
	}
}

func TestLightTextMeetsContrast(t *testing.T) {
	th := Light()
	if ContrastRatio(th.Text, th.Bg) < 4.5 {
		t.Fatalf("text contrast %f", ContrastRatio(th.Text, th.Bg))
	}
	if ContrastRatio(th.Muted, th.Bg) < 4.5 {
		t.Fatalf("muted contrast %f", ContrastRatio(th.Muted, th.Bg))
	}
	if ContrastRatio(th.Caption, th.Card) < 4.5 {
		t.Fatalf("caption contrast %f", ContrastRatio(th.Caption, th.Card))
	}
}
