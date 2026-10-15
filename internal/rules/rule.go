// Package rules loads cleanup descriptions. A rule can name a known target
// and a known action. It cannot add paths to the safety allowlist or run a script.
package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	_ "embed"
)

//go:embed builtin.json
var builtinJSON []byte

// Rule is one cleanup item. Paths are tokens, not free-form filesystem paths.
type Rule struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Group         string   `json:"group"`
	Risk          int      `json:"risk"`
	DefaultOn     bool     `json:"defaultOn"`
	MinWindows    int      `json:"minWindows"`
	Tokens        []string `json:"tokens,omitempty"`
	SkipIfProcess []string `json:"skipIfProcess,omitempty"`
	MinAgeHours   int      `json:"minAgeHours,omitempty"`
	AgeReason     string   `json:"ageReason,omitempty"`
	Warning       string   `json:"warning,omitempty"`
	HideIfMissing bool     `json:"hideIfMissing,omitempty"`
	NeedsAdmin    bool     `json:"needsAdmin"`
	Action        string   `json:"action"`
	Preview       bool     `json:"preview"`
	CloseApp      bool     `json:"closeApp"`
	Version       int      `json:"version"`
}

var (
	actions = map[string]bool{
		"files": true, "recycle": true, "dism": true, "winupdate": true, "windowsold": true,
	}
	groups = map[string]bool{
		"regular": true, "deep": true, "system": true, "recycle": true,
	}
	tokens = map[string]bool{
		"user-temp": true, "windows-temp": true, "thumbnail": true, "inet": true, "wer": true,
		"browser-chrome": true, "browser-edge": true, "browser-firefox": true, "browser-other": true,
		"crash": true, "shader": true, "winlogs": true, "devcache": true, "delivery": true,
		"winupdate": true, "windows-old": true,
	}
	processes = map[string]bool{
		"chrome.exe": true, "msedge.exe": true, "firefox.exe": true, "brave.exe": true, "opera.exe": true,
	}
)

// Builtin returns the shipped rules. They are rejected as a set if any rule is unsafe.
func Builtin() ([]Rule, error) {
	return Parse(builtinJSON)
}

// Parse decodes rules and rejects anything outside the supported action set.
// Unknown JSON fields are rejected so a rule cannot smuggle a command.
func Parse(data []byte) ([]Rule, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var list []Rule
	if err := dec.Decode(&list); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("规则文件包含多余内容")
	}
	seen := map[string]bool{}
	for i := range list {
		if err := validate(&list[i]); err != nil {
			return nil, fmt.Errorf("规则 %s: %w", list[i].ID, err)
		}
		if seen[list[i].ID] {
			return nil, fmt.Errorf("重复的规则 %s", list[i].ID)
		}
		seen[list[i].ID] = true
	}
	return list, nil
}

func validate(rule *Rule) error {
	if strings.TrimSpace(rule.ID) == "" || strings.TrimSpace(rule.Name) == "" {
		return errors.New("缺少编号或名称")
	}
	if !groups[rule.Group] {
		return errors.New("未知分类")
	}
	if rule.Risk < 1 || rule.Risk > 3 {
		return errors.New("风险等级必须是 1、2 或 3")
	}
	if rule.Risk >= 2 && rule.DefaultOn {
		return errors.New("深度清理和系统清理不能默认勾选")
	}
	if rule.ID == "recycle" && rule.DefaultOn {
		return errors.New("回收站不能默认勾选")
	}
	if rule.Group == "system" && rule.DefaultOn {
		return errors.New("系统清理不能默认勾选")
	}
	if !actions[rule.Action] {
		return errors.New("未知或未启用的执行方式")
	}
	if rule.CloseApp {
		return errors.New("规则不能要求结束进程")
	}
	if rule.Version < 1 {
		return errors.New("缺少规则版本")
	}
	if rule.MinAgeHours < 0 {
		return errors.New("文件年龄不能为负数")
	}
	if usesOrdinaryTemp(rule.Tokens) && rule.MinAgeHours < 72 {
		return errors.New("普通临时目录至少保留 72 小时")
	}
	if rule.Action == "files" && rule.MinAgeHours < 72 && strings.TrimSpace(rule.AgeReason) == "" {
		return errors.New("短于 72 小时的规则必须说明原因，年龄本身不能作为删除依据")
	}
	if rule.MinWindows < 10 {
		return errors.New("只支持 Windows 10 及更高版本")
	}
	if rule.Action == "files" && len(rule.Tokens) == 0 {
		return errors.New("文件清理必须指定已知目标")
	}
	if rule.Action != "files" && rule.Action != "winupdate" && rule.Action != "windowsold" && len(rule.Tokens) > 0 {
		return errors.New("该执行方式不能附带目录")
	}
	for _, token := range rule.Tokens {
		if !tokens[token] {
			return fmt.Errorf("未知目标 %s", token)
		}
	}
	for _, name := range rule.SkipIfProcess {
		if !processes[strings.ToLower(name)] {
			return fmt.Errorf("不能监视进程 %s", name)
		}
	}
	if slices.Contains(rule.Tokens, "windows-old") && rule.Action != "windowsold" {
		return errors.New("旧版 Windows 只能走系统清理接口")
	}
	return nil
}

func usesOrdinaryTemp(tokens []string) bool {
	for _, token := range tokens {
		if token == "user-temp" || token == "windows-temp" {
			return true
		}
	}
	return false
}
