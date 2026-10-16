package apps

import "context"

// App is one registered uninstall entry. Size is what Windows recorded, not a measured tree.
type App struct {
	Name        string
	Version     string
	Publisher   string
	Location    string
	Uninstall   string
	Estimated   int64
	EstimatedOK bool
	Note        string
	System      bool
	Kind        string
}

// Report lists registered applications. It never deletes install directories.
type Report struct {
	Apps      []App
	Note      string
	Errors    []string
	Cancelled bool
}

// ListInstalled is filled on Windows. Tests and other OS get an empty report.
var ListInstalled = func(ctx context.Context) Report {
	return Report{Note: "已安装软件列表只能在 Windows 上从卸载登记读取。"}
}
