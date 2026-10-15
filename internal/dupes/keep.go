package dupes

import (
	"errors"
	"strings"

	"cclear/internal/space"
)

// CanRemove reports whether one copy may go to the recycle bin.
// At least one other copy in the same content group must remain.
func CanRemove(groups []Group, path string) error {
	if path == "" {
		return errors.New("没有选择文件")
	}
	if space.HighRiskDelete(path) {
		return errors.New("系统文件、程序目录或虚拟磁盘不能从这里删除")
	}
	for _, g := range groups {
		matched := 0
		for _, file := range g.Files {
			if samePath(file.Path, path) {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		if len(g.Files)-matched < 1 {
			return errors.New("必须至少保留一个副本")
		}
		return nil
	}
	return errors.New("该文件不在已确认的重复组中，不能从这里删除")
}

func samePath(a, b string) bool {
	return strings.EqualFold(strings.ReplaceAll(a, "/", `\`), strings.ReplaceAll(b, "/", `\`))
}
