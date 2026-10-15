package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cclear/internal/dupes"
	"cclear/internal/space"
)

func main() {
	root, err := os.MkdirTemp("", "cclear-shots-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(root)
	dirs := []string{"Windows", "Users", "ProgramData", "Temp"}
	for i, name := range dirs {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		payload := make([]byte, (i+1)*32*1024)
		if err := os.WriteFile(filepath.Join(dir, name+".bin"), payload, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	dup := make([]byte, 80<<10)
	if err := os.WriteFile(filepath.Join(root, "Temp", "copy-a.bin"), dup, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(root, "Temp", "copy-b.bin"), dup, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rep := space.Scan(context.Background(), root, nil)
	drep := dupes.Find(context.Background(), root, nil)
	outDir := "dist/screenshots"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	img := space.RenderUIPreview(rep, 256<<20, 80<<20)
	pngPath := filepath.Join(outDir, "analyze-real-scan.png")
	if err := space.WritePNG(pngPath, img); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	note := filepath.Join(outDir, "README.txt")
	text := "analyze-real-scan.png is generated from a real temporary filesystem scan.\r\n" +
		"It is a layout sketch with real sizes, not a Win32 window screenshot.\r\n" +
		"Windows GUI screenshots: unverified on this Darwin host.\r\n" +
		fmt.Sprintf("files=%d logical=%d allocated=%d\r\n", rep.Files, rep.Logical, rep.Allocated) +
		fmt.Sprintf("duplicate groups=%d extra bytes=%d truncated=%v\r\n", len(drep.Groups), drep.Bytes, drep.Truncated)
	if err := os.WriteFile(note, []byte(text), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s files=%d allocated=%d\n", pngPath, rep.Files, rep.Allocated)
}
