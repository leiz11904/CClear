package clean

// InitialCheck is the checkbox used the first time a category is shown.
// Empty categories stay off so a scan does not look like it selected nothing useful.
func InitialCheck(defaultOn bool, bytes int64, unknown bool) bool {
	return defaultOn && (bytes > 0 || unknown)
}

// StickyChoice reports whether a category may be remembered across launches.
// Mid-risk, high-risk, and recycle must be chosen again every session.
func StickyChoice(id string, risk int) bool {
	if risk >= 2 || id == "recycle" || id == "dism" || id == "winupdate" || id == "windowsold" {
		return false
	}
	return true
}

// ClampChoices turns off persisted dangerous checks from older settings files.
func ClampChoices(choices map[string]bool, riskOf map[string]int) {
	if choices == nil {
		return
	}
	for id, on := range choices {
		if !on {
			continue
		}
		if !StickyChoice(id, riskOf[id]) {
			choices[id] = false
		}
	}
}

// RememberedCheck keeps a choice the user already made in this session.
// Dangerous categories are never restored from a previous launch's saved file
// after ClampChoices; in-session map values still apply.
func RememberedCheck(id string, defaultOn bool, bytes int64, unknown bool, chosen map[string]bool) bool {
	if chosen != nil {
		if on, ok := chosen[id]; ok {
			return on
		}
	}
	return InitialCheck(defaultOn, bytes, unknown)
}

// AllSelected reports whether every allowed row is checked.
func AllSelected(current []bool) bool {
	return AllSelectedAllowed(current, nil)
}

func AllSelectedAllowed(current, allowed []bool) bool {
	if len(current) == 0 {
		return false
	}
	any := false
	for i, on := range current {
		if allowed != nil && i < len(allowed) && !allowed[i] {
			continue
		}
		any = true
		if !on {
			return false
		}
	}
	return any
}

// SelectAllLabel is the button title for the current selection.
func SelectAllLabel(allSelected bool) string {
	if allSelected {
		return "取消全选"
	}
	return "全选"
}

// ToggleChecks checks every row, or clears them when they are already all checked.
func ToggleChecks(current []bool) []bool {
	return ToggleChecksAllowed(current, nil)
}

// ToggleChecksAllowed never turns on rows with allowed[i]==false.
// 取消全选 still clears every row, including dangerous ones.
func ToggleChecksAllowed(current, allowed []bool) []bool {
	next := make([]bool, len(current))
	if len(current) == 0 {
		return next
	}
	allOn := AllSelectedAllowed(current, allowed)
	on := !allOn
	for i := range next {
		if !on {
			next[i] = false
			continue
		}
		if allowed != nil && i < len(allowed) && !allowed[i] {
			next[i] = current[i]
			continue
		}
		next[i] = true
	}
	return next
}

// KeepPreviousScan reports that a cancelled or failed rescan should leave the
// list the user was looking at in place.
func KeepPreviousScan(cancelled bool, previousCount int) bool {
	return cancelled && previousCount > 0
}
