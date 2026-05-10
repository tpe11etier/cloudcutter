package config

import (
	"os"
	"regexp"
	"strings"
)

// ExpandEnv replaces ${VAR} and ${VAR:-default} sequences in s with the
// value of VAR (empty string when unset and no default; the default when
// unset OR set-but-empty). Literal `$$` passes through unchanged so users
// can write a literal dollar-sign without escaping.
//
// Anything else starting with `$` is left alone. This is intentionally
// narrower than os.ExpandEnv, which matches `$VAR` without braces and
// surprises users who write paths like `~/$HOME-of-fame`.
func ExpandEnv(s string) string {
	// Replace `$$` with a sentinel that won't appear in input, run the
	// expansion, then restore.
	const sentinel = "\x00CC_DOLLAR\x00"
	s = strings.ReplaceAll(s, "$$", sentinel)
	s = envVarRE.ReplaceAllStringFunc(s, func(match string) string {
		// match looks like ${NAME} or ${NAME:-default}
		inner := match[2 : len(match)-1]
		name, def, hasDef := strings.Cut(inner, ":-")
		val, ok := os.LookupEnv(name)
		if !ok || val == "" {
			if hasDef {
				return def
			}
			return ""
		}
		return val
	})
	return strings.ReplaceAll(s, sentinel, "$$")
}

var envVarRE = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*(?::-[^}]*)?\}`)
