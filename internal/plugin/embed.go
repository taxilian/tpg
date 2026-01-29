// Package plugin provides the embedded OpenCode plugin source.
package plugin

import _ "embed"

//go:embed opencode.ts
var OpencodeSource string
