package rules

import "testing"

func TestBuiltinAudit(t *testing.T) {
	list, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 18 {
		t.Fatalf("rules = %d", len(list))
	}
	type expect struct {
		group  string
		risk   int
		on     bool
		action string
		age    int
	}
	want := map[string]expect{
		"user-temp":       {"regular", 1, true, "files", 72},
		"windows-temp":    {"regular", 1, true, "files", 72},
		"thumbnail":       {"regular", 1, true, "files", 0},
		"inet":            {"regular", 1, true, "files", 0},
		"wer":             {"regular", 1, false, "files", 0},
		"browser-chrome":  {"regular", 1, true, "files", 0},
		"browser-edge":    {"regular", 1, true, "files", 0},
		"browser-firefox": {"regular", 1, true, "files", 0},
		"browser-other":   {"regular", 1, true, "files", 0},
		"recycle":         {"recycle", 1, false, "recycle", 0},
		"crash":           {"deep", 2, false, "files", 0},
		"shader":          {"deep", 2, false, "files", 0},
		"winlogs":         {"deep", 2, false, "files", 0},
		"devcache":        {"deep", 2, false, "files", 0},
		"delivery":        {"deep", 2, false, "files", 0},
		"winupdate":       {"system", 3, false, "winupdate", 0},
		"windowsold":      {"system", 3, false, "windowsold", 0},
		"dism":            {"system", 3, false, "dism", 0},
	}
	seen := map[string]bool{}
	for _, rule := range list {
		exp, ok := want[rule.ID]
		if !ok {
			t.Errorf("unexpected rule %s", rule.ID)
			continue
		}
		seen[rule.ID] = true
		if rule.Group != exp.group || rule.Risk != exp.risk || rule.DefaultOn != exp.on || rule.Action != exp.action || rule.MinAgeHours != exp.age {
			t.Errorf("%s = group %s risk %d on %v action %s age %d", rule.ID, rule.Group, rule.Risk, rule.DefaultOn, rule.Action, rule.MinAgeHours)
		}
		if rule.Action == "files" && rule.MinAgeHours < 72 && rule.AgeReason == "" {
			t.Errorf("%s has no reason for a short age", rule.ID)
		}
		if rule.CloseApp {
			t.Errorf("%s closes an application", rule.ID)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("audited %d", len(seen))
	}
}
