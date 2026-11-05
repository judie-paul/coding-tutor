package view

import (
	"fmt"
	"html/template"
	"strings"
	"time"
	"unicode"
)

// Funcs returns the helper functions available in every template.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"markdown":   Markdown,
		"timeAgo":    TimeAgo,
		"date":       func(t time.Time) string { return t.Local().Format("2 Jan 2006") },
		"dateTime":   func(t time.Time) string { return t.Local().Format("2 Jan 2006, 15:04") },
		"isoTime":    func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
		"capitalize": Capitalize,
		"humanize":   func(s string) string { return Capitalize(strings.ReplaceAll(s, "_", " ")) },
		"add":        func(a, b int) int { return a + b },
		"plural": func(n int, singular, plural string) string {
			if n == 1 {
				return fmt.Sprintf("%d %s", n, singular)
			}
			return fmt.Sprintf("%d %s", n, plural)
		},
		"lines": func(s string) []string {
			var out []string
			for _, l := range strings.Split(s, "\n") {
				if l = strings.TrimSpace(l); l != "" {
					out = append(out, l)
				}
			}
			return out
		},
		"initial": func(s string) string {
			for _, r := range s {
				return string(unicode.ToUpper(r))
			}
			return "?"
		},
		"dict": func(kv ...any) map[string]any {
			m := make(map[string]any, len(kv)/2)
			for i := 0; i+1 < len(kv); i += 2 {
				if k, ok := kv[i].(string); ok {
					m[k] = kv[i+1]
				}
			}
			return m
		},
		"contains": func(list []string, s string) bool {
			for _, v := range list {
				if v == s {
					return true
				}
			}
			return false
		},
	}
}

// Capitalize upper-cases the first letter of s.
func Capitalize(s string) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}

// TimeAgo describes how long ago t was, e.g. "5 minutes ago".
func TimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return pluralize(int(d.Minutes()), "minute") + " ago"
	case d < 24*time.Hour:
		return pluralize(int(d.Hours()), "hour") + " ago"
	case d < 30*24*time.Hour:
		return pluralize(int(d.Hours()/24), "day") + " ago"
	default:
		return t.Local().Format("2 Jan 2006")
	}
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
