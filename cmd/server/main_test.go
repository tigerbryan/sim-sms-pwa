package main

import (
	"strings"
	"testing"

	"qmi-web/internal/version"
)

func TestVersionTextUsesCurrentBuildMetadata(t *testing.T) {
	text := versionText()
	for _, value := range []string{version.Current().Version, version.Current().Commit, version.Current().BuildTime} {
		if !strings.Contains(text, value) {
			t.Fatalf("version text %q does not contain %q", text, value)
		}
	}
}
