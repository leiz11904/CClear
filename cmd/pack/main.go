package main

import (
	"fmt"
	"os"

	"cclear/internal/payload"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: pack installer.exe payload.exe")
		os.Exit(2)
	}
	stub, err := os.ReadFile(os.Args[1])
	if err != nil {
		fatal(err)
	}
	app, err := os.ReadFile(os.Args[2])
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(os.Args[1], payload.Append(stub, app), 0o755); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
