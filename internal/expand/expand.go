// Package expand implements ${VAR} substitution for commands and env files.
package expand

import (
	"os"
	"regexp"
)

var varPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Vars replaces every ${VAR} reference in s using lookup. References that
// lookup cannot resolve are left untouched, so downstream shells can still
// expand them. Only the braced form is recognized; a bare $VAR is passed
// through as-is.
func Vars(s string, lookup func(key string) (string, bool)) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-1]
		if value, ok := lookup(key); ok {
			return value
		}
		return match
	})
}

// OSLookup returns a lookup func backed by the process environment. Keys in
// extra take precedence over environment variables.
func OSLookup(extra map[string]string) func(key string) (string, bool) {
	return func(key string) (string, bool) {
		if value, ok := extra[key]; ok {
			return value, true
		}
		return os.LookupEnv(key)
	}
}
