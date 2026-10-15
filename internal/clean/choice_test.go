package clean

import "testing"

func TestInitialCheck(t *testing.T) {
	if !InitialCheck(true, 1024, false) {
		t.Fatal("default item with data should start checked")
	}
	if InitialCheck(true, 0, false) {
		t.Fatal("empty default item should start unchecked")
	}
	if InitialCheck(false, 300<<20, false) {
		t.Fatal("optional item should start unchecked even when it has data")
	}
	if !InitialCheck(true, -1, true) {
		t.Fatal("default item measured by the system should start checked")
	}
	if InitialCheck(false, -1, true) {
		t.Fatal("optional system-measured item should start unchecked")
	}
}

func TestRescanKeepsManualSelection(t *testing.T) {
	chosen := map[string]bool{
		"thumbnail": false,
		"winlogs":   true,
	}
	if RememberedCheck("thumbnail", true, 35<<20, false, chosen) {
		t.Fatal("rescan checked a category the user had turned off")
	}
	if !RememberedCheck("winlogs", false, 34<<20, false, chosen) {
		t.Fatal("rescan cleared a category the user had turned on")
	}
	if RememberedCheck("shader", false, 303<<20, false, chosen) {
		t.Fatal("an untouched optional category became checked")
	}
	if !RememberedCheck("user-temp", true, 1<<30, false, chosen) {
		t.Fatal("an untouched default category with data became unchecked")
	}
	if RememberedCheck("inet", true, 0, false, nil) {
		t.Fatal("first scan should not check an empty category")
	}
	if RememberedCheck("windowsold", false, 2<<30, false, chosen) {
		t.Fatal("a new optional category should stay off until the user checks it")
	}
}

func TestClampChoicesClearsHighRisk(t *testing.T) {
	choices := map[string]bool{"user-temp": true, "dism": true, "winupdate": true, "recycle": true, "winlogs": true}
	ClampChoices(choices, map[string]int{"user-temp": 1, "dism": 3, "winupdate": 3, "recycle": 1, "winlogs": 2})
	if !choices["user-temp"] {
		t.Fatal("low risk was cleared")
	}
	if choices["dism"] || choices["winupdate"] || choices["recycle"] || choices["winlogs"] {
		t.Fatalf("%v", choices)
	}
}

func TestToggleChecksSkipsHighRisk(t *testing.T) {
	current := []bool{false, false, true}
	allowed := []bool{true, false, true}
	got := ToggleChecksAllowed(current, allowed)
	if !got[0] || got[1] || !got[2] {
		t.Fatalf("%v", got)
	}
	cleared := ToggleChecksAllowed(got, allowed)
	if cleared[0] || cleared[1] || cleared[2] {
		t.Fatalf("cancel-all %v", cleared)
	}
}

func TestToggleChecks(t *testing.T) {
	mixed := ToggleChecks([]bool{true, false, true})
	if !AllSelected(mixed) || SelectAllLabel(false) != "全选" {
		t.Fatal("mixed selection should become all checked and offer 全选")
	}
	cleared := ToggleChecks(mixed)
	if AllSelected(cleared) || SelectAllLabel(true) != "取消全选" {
		t.Fatal("a second press should clear every row and offer 取消全选")
	}
	for _, on := range cleared {
		if on {
			t.Fatal("cancel-all left a row checked")
		}
	}
	if AllSelected(nil) {
		t.Fatal("empty list must not count as all selected")
	}
	if len(ToggleChecks(nil)) != 0 {
		t.Fatal("empty list should stay empty")
	}
}

func TestKeepPreviousScan(t *testing.T) {
	if !KeepPreviousScan(true, 3) {
		t.Fatal("cancelling a rescan should keep the previous list")
	}
	if KeepPreviousScan(true, 0) {
		t.Fatal("cancelling the first scan has no previous list to keep")
	}
	if KeepPreviousScan(false, 3) {
		t.Fatal("a finished rescan should replace the list")
	}
}
