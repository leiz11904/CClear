package space

import (
	"errors"
	"strconv"
	"strings"
)

// ParseSizeMB parses a whole number of mebibytes entered by the user.
func ParseSizeMB(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("请输入以 MB 为单位的正整数")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n > 1048576 {
		return 0, errors.New("请输入 1 到 1048576 之间的整数（MB）")
	}
	return n << 20, nil
}
