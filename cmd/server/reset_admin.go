package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"qmi-web/internal/auth"
	"qmi-web/internal/database"
	"golang.org/x/term"
)

// resetAdmin is intentionally a NAS-local recovery command. It accepts no
// password argument, so a password cannot be placed in shell history.
func resetAdmin(args []string) {
	flags := flag.NewFlagSet("reset-admin", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dataDir := flags.String("data", envOr("QMI_WEB_DATA", "/data"), "persistent data directory")
	resetDefault := flags.Bool("reset-default", false, "reset the administrator to the documented default password")
	if err := flags.Parse(args); err != nil {
		os.Exit(2)
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "qmi-web reset-admin: no positional arguments are accepted")
		os.Exit(2)
	}

	store, err := database.Open(filepath.Join(*dataDir, "qmi-web.db"))
	if err != nil {
		fatal("open database", err)
	}
	defer store.Close()
	service := auth.New(store, *dataDir, false)
	if *resetDefault {
		if err := service.ResetAdminPassword(context.Background(), auth.DefaultPassword); err != nil {
			fatal("reset administrator password", err)
		}
		fmt.Fprintln(os.Stdout, "administrator password reset; all sessions were invalidated")
		return
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fmt.Fprintln(os.Stderr, "qmi-web reset-admin: an interactive NAS terminal is required; use --reset-default only when necessary")
		os.Exit(2)
	}
	first, err := readPassword("New administrator password: ")
	if err != nil {
		fatal("read new password", err)
	}
	defer clearPassword(first)
	second, err := readPassword("Confirm new administrator password: ")
	if err != nil {
		fatal("read password confirmation", err)
	}
	defer clearPassword(second)
	if string(first) != string(second) {
		fmt.Fprintln(os.Stderr, "qmi-web reset-admin: password confirmation does not match")
		os.Exit(2)
	}
	if err := service.ResetAdminPassword(context.Background(), string(first)); err != nil {
		fatal("reset administrator password", err)
	}
	fmt.Fprintln(os.Stdout, "administrator password updated; all sessions were invalidated")
}

func readPassword(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return value, err
}

func clearPassword(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
