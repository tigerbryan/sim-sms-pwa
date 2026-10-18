//go:build !linux

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "qmi-probe is available only on Linux")
	os.Exit(2)
}
