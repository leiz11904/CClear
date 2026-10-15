package clean

import (
	"strings"
	"time"

	"cclear/internal/rules"
	"cclear/internal/winpath"
)

// Env is the machine layout used to build cleanup roots.
type Env struct {
	SystemDrive  string
	SystemRoot   string
	LocalAppData string
	ProgramData  string
	Temp         string
	UserProfile  string
}

// Category is one selectable cleanup item.
type Category struct {
	ID            string
	Name          string
	Description   string
	Warning       string
	Group         string
	Risk          int
	NeedsAdmin    bool
	DefaultOn     bool
	HideIfMissing bool
	MinAge        time.Duration
	RemoveRoot    bool
	Special       string
	Preview       bool
	SkipProcesses []string
	Roots         []string
	Discover      func() []string
}

// Categories returns the product cleanup list. Targets come from the rule
// file, and every resolved path still has to pass the safety allowlist.
func Categories(env Env) []Category {
	list, err := rules.Builtin()
	if err != nil {
		return nil
	}
	return categoriesFrom(env, list)
}

func categoriesFrom(env Env, list []rules.Rule) []Category {
	drive := normalizeDrive(env.SystemDrive)
	var cats []Category
	for _, rule := range list {
		if rule.MinWindows > 10 && windowsMajor() < rule.MinWindows {
			continue
		}
		cat := Category{
			ID:            rule.ID,
			Name:          rule.Name,
			Description:   rule.Description,
			Warning:       rule.Warning,
			Group:         rule.Group,
			Risk:          rule.Risk,
			NeedsAdmin:    rule.NeedsAdmin,
			DefaultOn:     rule.DefaultOn,
			HideIfMissing: rule.HideIfMissing,
			MinAge:        time.Duration(rule.MinAgeHours) * time.Hour,
			Special:       specialAction(rule.Action),
			Preview:       rule.Preview,
			SkipProcesses: append([]string(nil), rule.SkipIfProcess...),
			Roots:         resolveTokens(env, drive, rule.Tokens),
		}
		cats = append(cats, cat)
	}
	return cats
}

func specialAction(action string) string {
	switch action {
	case "files":
		return ""
	default:
		return action
	}
}

func resolveTokens(env Env, drive string, tokens []string) []string {
	windows := windowsDir(env, drive)
	local := cleanJoin(env.LocalAppData)
	programData := cleanJoin(env.ProgramData)
	temp := cleanJoin(env.Temp)
	var paths []string
	for _, token := range tokens {
		switch token {
		case "user-temp":
			paths = append(paths, temp, slash(local, "Temp"))
		case "windows-temp":
			paths = append(paths, slash(windows, "Temp"))
		case "thumbnail":
			paths = append(paths, slash(local, `Microsoft\Windows\Explorer`))
		case "inet":
			paths = append(paths, slash(local, `Microsoft\Windows\INetCache`))
		case "wer":
			paths = append(paths,
				slash(local, `Microsoft\Windows\WER\ReportArchive`),
				slash(local, `Microsoft\Windows\WER\ReportQueue`),
				slash(programData, `Microsoft\Windows\WER\ReportArchive`),
				slash(programData, `Microsoft\Windows\WER\ReportQueue`),
			)
		case "browser-chrome":
			paths = append(paths, toWindowsPaths(DiscoverFamily(local, "chrome"))...)
		case "browser-edge":
			paths = append(paths, toWindowsPaths(DiscoverFamily(local, "edge"))...)
		case "browser-firefox":
			paths = append(paths, toWindowsPaths(DiscoverFamily(local, "firefox"))...)
		case "browser-other":
			paths = append(paths, toWindowsPaths(DiscoverFamily(local, "other"))...)
		case "crash":
			paths = append(paths, slash(local, "CrashDumps"), slash(windows, "Minidump"), slash(windows, "MEMORY.DMP"))
		case "shader":
			paths = append(paths,
				slash(local, "D3DSCache"),
				slash(local, `NVIDIA\DXCache`),
				slash(local, `NVIDIA\GLCache`),
				slash(local, `NVIDIA Corporation\NV_Cache`),
				slash(local, `AMD\DxCache`),
				slash(local, `AMD\GLCache`),
			)
		case "winlogs":
			paths = append(paths, slash(windows, `Logs\CBS`), slash(windows, `Logs\DISM`))
		case "devcache":
			paths = append(paths,
				slash(local, "npm-cache"),
				slash(local, "pnpm-cache"),
				slash(local, `Yarn\Cache`),
				slash(local, `pip\Cache`),
				slash(local, `NuGet\v3-cache`),
				slash(local, `NuGet\http-cache`),
				slash(local, "go-build"),
			)
		case "delivery":
			paths = append(paths,
				slash(windows, `ServiceProfiles\NetworkService\AppData\Local\Microsoft\Windows\DeliveryOptimization\Cache`),
				slash(programData, `Microsoft\Windows\DeliveryOptimization\Cache`),
			)
		case "winupdate":
			paths = append(paths, slash(windows, `SoftwareDistribution\Download`))
		case "windows-old":
			paths = append(paths, slash(drive, "Windows.old"))
		}
	}
	return allowedRoots(drive, paths...)
}

var windowsMajor = func() int { return 10 }

func normalizeDrive(drive string) string {
	drive = strings.TrimSpace(drive)
	drive = strings.TrimRight(drive, `\/`)
	if len(drive) >= 2 && drive[1] == ':' {
		return strings.ToUpper(drive[:2])
	}
	return "C:"
}

func windowsDir(env Env, drive string) string {
	root := cleanJoin(env.SystemRoot)
	if strings.EqualFold(root, drive+`\Windows`) {
		return drive + `\Windows`
	}
	return drive + `\Windows`
}

func cleanJoin(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "/", `\`)
	cleaned, err := winpath.Clean(path)
	if err != nil {
		return ""
	}
	return cleaned
}

func slash(parts ...string) string {
	var out []string
	for _, part := range parts {
		part = strings.ReplaceAll(part, "/", `\`)
		part = strings.Trim(part, `\`)
		if part == "" {
			continue
		}
		for _, piece := range strings.Split(part, `\`) {
			if piece != "" {
				out = append(out, piece)
			}
		}
	}
	return strings.Join(out, `\`)
}

func allowedRoots(drive string, paths ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, path := range paths {
		cleaned, err := winpath.Clean(path)
		if err != nil {
			continue
		}
		key := strings.ToLower(cleaned)
		if seen[key] {
			continue
		}
		if err := winpath.ValidateRoot(cleaned, drive); err != nil {
			continue
		}
		seen[key] = true
		out = append(out, cleaned)
	}
	return out
}

func toWindowsPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, strings.ReplaceAll(path, "/", `\`))
	}
	return out
}

// Visible reports whether a category should appear before scanning.
func (c Category) Visible() bool {
	if !c.HideIfMissing {
		return true
	}
	for _, root := range c.Roots {
		if realDirPath(root) {
			return true
		}
	}
	return false
}
