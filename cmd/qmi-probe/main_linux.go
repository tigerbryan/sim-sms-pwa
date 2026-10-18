//go:build linux

// qmi-probe performs the narrowest possible QMI device permission check.
// It opens exactly one cdc-wdm character node read/write, fstats it, closes it,
// and sends no QMI request or any other modem command.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: qmi-probe /dev/cdc-wdmX")
		os.Exit(2)
	}
	path := filepath.Clean(os.Args[1])
	if filepath.Dir(path) != "/dev" || !strings.HasPrefix(filepath.Base(path), "cdc-wdm") {
		fmt.Fprintln(os.Stderr, "qmi-probe: target must be one /dev/cdc-wdmX node")
		os.Exit(2)
	}
	info, err := os.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "qmi-probe: lstat: %v\n", err)
		os.Exit(1)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		fmt.Fprintln(os.Stderr, "qmi-probe: target is not a character device")
		os.Exit(1)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "qmi-probe: open: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "qmi-probe: fstat: %v\n", err)
		os.Exit(1)
	}
	stat, ok := opened.Sys().(*syscall.Stat_t)
	if !ok || opened.Mode()&os.ModeCharDevice == 0 {
		fmt.Fprintln(os.Stderr, "qmi-probe: fstat did not return a character device")
		os.Exit(1)
	}
	rdev := uint64(stat.Rdev)
	major := ((rdev >> 8) & 0xfff) | ((rdev >> 32) & 0xfffff000)
	minor := (rdev & 0xff) | ((rdev >> 12) & 0xffffff00)
	fmt.Printf("qmi-probe: open/fstat/close passed (major=%d minor=%d)\n", major, minor)
}
