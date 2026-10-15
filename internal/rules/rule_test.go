package rules

import "testing"

func TestBuiltinRulesAreSafe(t *testing.T) {
	list, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 15 {
		t.Fatalf("rules = %d", len(list))
	}
	for _, rule := range list {
		if rule.Risk >= 2 && rule.DefaultOn {
			t.Errorf("%s is risky and selected by default", rule.ID)
		}
		if rule.CloseApp {
			t.Errorf("%s wants to kill a process", rule.ID)
		}
	}
}

func TestRecycleCrashAndSystemStayOff(t *testing.T) {
	list, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	off := map[string]bool{}
	for _, rule := range list {
		off[rule.ID] = rule.DefaultOn
	}
	for _, id := range []string{"recycle", "wer", "crash", "shader", "winlogs", "devcache", "delivery", "winupdate", "windowsold", "dism"} {
		if off[id] {
			t.Errorf("%s is on by default", id)
		}
	}
	for _, id := range []string{"user-temp", "windows-temp", "thumbnail", "browser-chrome"} {
		if !off[id] {
			t.Errorf("%s should start on", id)
		}
	}
}

func TestRejectsScriptsAndUnknownTargets(t *testing.T) {
	cases := []string{
		`[{"id":"x","name":"x","description":"x","group":"regular","risk":1,"defaultOn":false,"minWindows":10,"action":"files","preview":true,"closeApp":false,"version":1,"command":"cmd.exe"}]`,
		`[{"id":"x","name":"x","description":"x","group":"regular","risk":1,"defaultOn":false,"minWindows":10,"tokens":["documents"],"action":"files","preview":true,"closeApp":false,"version":1}]`,
		`[{"id":"x","name":"x","description":"x","group":"deep","risk":2,"defaultOn":true,"minWindows":10,"tokens":["shader"],"action":"files","preview":true,"closeApp":false,"version":1}]`,
		`[{"id":"recycle","name":"回收站","description":"x","group":"recycle","risk":1,"defaultOn":true,"minWindows":10,"action":"recycle","preview":false,"closeApp":false,"version":1}]`,
		`[{"id":"x","name":"x","description":"x","group":"regular","risk":1,"defaultOn":false,"minWindows":10,"tokens":["windows-old"],"action":"files","preview":true,"closeApp":false,"version":1}]`,
		`[{"id":"x","name":"x","description":"x","group":"regular","risk":1,"defaultOn":false,"minWindows":10,"action":"powershell","preview":true,"closeApp":false,"version":1}]`,
		`[{"id":"x","name":"x","description":"x","group":"regular","risk":1,"defaultOn":false,"minWindows":10,"tokens":["user-temp"],"action":"files","preview":true,"closeApp":true,"version":1}]`,
		`[{"id":"x","name":"x","description":"x","group":"regular","risk":1,"defaultOn":true,"minWindows":10,"tokens":["user-temp"],"minAgeHours":24,"action":"files","preview":true,"closeApp":false,"version":1}]`,
		`[{"id":"x","name":"x","description":"x","group":"regular","risk":1,"defaultOn":true,"minWindows":10,"tokens":["thumbnail"],"action":"files","preview":true,"closeApp":false,"version":1}]`,
	}
	for _, body := range cases {
		if _, err := Parse([]byte(body)); err == nil {
			t.Fatalf("accepted unsafe rule: %s", body)
		}
	}
}
