// Package web embeds the HTML templates and static assets into the binary.
package web

import "embed"

// FS contains templates/ and static/.
//
//go:embed templates static
var FS embed.FS
