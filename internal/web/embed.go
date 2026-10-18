// Package web embeds the production Vue build into the Go binary.
package web

import "embed"

//go:embed dist/*
var Assets embed.FS
