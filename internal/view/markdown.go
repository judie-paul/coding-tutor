package view

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

// Markdown converts a small, safe subset of Markdown (as produced by AI
// models) into HTML. All input text is HTML-escaped before any formatting is
// applied, so model output can never inject markup or scripts.
//
// Supported: fenced code blocks, headings, bullet and numbered lists,
// block quotes, paragraphs, inline code, bold and italic.
func Markdown(src string) template.HTML {
	var out strings.Builder
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")

	var para []string
	listType := "" // "ul" or "ol" when inside a list
	flushPara := func() {
		if len(para) > 0 {
			out.WriteString("<p>" + inline(strings.Join(para, " ")) + "</p>\n")
			para = nil
		}
	}
	closeList := func() {
		if listType != "" {
			out.WriteString("</" + listType + ">\n")
			listType = ""
		}
	}
	openList := func(t string) {
		if listType != t {
			closeList()
			out.WriteString("<" + t + ">\n")
			listType = t
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushPara()
			closeList()
			lang := sanitizeLang(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			out.WriteString(`<div class="code-block"><pre><code`)
			if lang != "" {
				out.WriteString(` class="language-` + lang + `" data-lang="` + lang + `"`)
			}
			out.WriteString(">" + html.EscapeString(strings.Join(code, "\n")) + "</code></pre></div>\n")
			continue
		}

		switch {
		case trimmed == "":
			flushPara()
			closeList()
		case strings.HasPrefix(trimmed, "#"):
			flushPara()
			closeList()
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			text := strings.TrimSpace(trimmed[level:])
			tag := "h3"
			if level >= 3 {
				tag = "h4"
			}
			out.WriteString("<" + tag + ">" + inline(text) + "</" + tag + ">\n")
		case bulletRe.MatchString(trimmed):
			flushPara()
			openList("ul")
			out.WriteString("<li>" + inline(bulletRe.ReplaceAllString(trimmed, "")) + "</li>\n")
		case orderedRe.MatchString(trimmed):
			flushPara()
			openList("ol")
			out.WriteString("<li>" + inline(orderedRe.ReplaceAllString(trimmed, "")) + "</li>\n")
		case strings.HasPrefix(trimmed, ">"):
			flushPara()
			closeList()
			out.WriteString("<blockquote>" + inline(strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))) + "</blockquote>\n")
		default:
			if listType != "" && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) {
				// Continuation of the previous list item: render as a separate line in the list.
				out.WriteString("<li class=\"cont\">" + inline(trimmed) + "</li>\n")
				continue
			}
			closeList()
			para = append(para, trimmed)
		}
	}
	flushPara()
	closeList()
	return template.HTML(out.String()) //nolint:gosec // all text above is escaped
}

var (
	bulletRe  = regexp.MustCompile(`^[-*+]\s+`)
	orderedRe = regexp.MustCompile(`^\d{1,3}[.)]\s+`)
	boldRe    = regexp.MustCompile(`\*\*([^*]+?)\*\*`)
	italicRe  = regexp.MustCompile(`(^|[\s(])\*([^*\s][^*]*?)\*`)
	langRe    = regexp.MustCompile(`^[A-Za-z0-9+#-]{1,20}$`)
)

func sanitizeLang(s string) string {
	if langRe.MatchString(s) {
		return strings.ToLower(s)
	}
	return ""
}

// inline escapes text and applies inline code, bold and italic formatting.
func inline(s string) string {
	var b strings.Builder
	for {
		start := strings.Index(s, "`")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+1:], "`")
		if end < 0 {
			break
		}
		b.WriteString(emphasis(html.EscapeString(s[:start])))
		b.WriteString("<code>" + html.EscapeString(s[start+1:start+1+end]) + "</code>")
		s = s[start+end+2:]
	}
	b.WriteString(emphasis(html.EscapeString(s)))
	return b.String()
}

func emphasis(escaped string) string {
	escaped = boldRe.ReplaceAllString(escaped, "<strong>$1</strong>")
	return italicRe.ReplaceAllString(escaped, "$1<em>$2</em>")
}
