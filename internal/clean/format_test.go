package clean

import "testing"

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		-1:                 "由系统计算",
		0:                  "0 B",
		1023:               "1023 B",
		1024:               "1 KB",
		1536:               "1.5 KB",
		1048576:            "1 MB",
		1024 * 1024 * 1024: "1 GB",
	}
	for n, want := range cases {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
