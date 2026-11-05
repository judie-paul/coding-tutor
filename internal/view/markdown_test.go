package view

import (
	"strings"
	"testing"
)

func TestMarkdownEscapesHTML(t *testing.T) {
	got := string(Markdown("Hello <script>alert('x')</script> **bold** <img src=x onerror=alert(1)>"))
	if strings.Contains(got, "<script>") || strings.Contains(got, "<img") {
		t.Fatalf("unescaped HTML in output: %s", got)
	}
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("bold not rendered: %s", got)
	}
}

func TestMarkdownBlocks(t *testing.T) {
	src := "# Title\n\nSome `code` and *emphasis*.\n\n- one\n- two\n\n1. first\n2. second\n\n```go\nfmt.Println(\"<hi>\")\n```\n\n> note"
	got := string(Markdown(src))
	for _, want := range []string{
		"<h3>Title</h3>",
		"<code>code</code>",
		"<em>emphasis</em>",
		"<ul>\n<li>one</li>\n<li>two</li>\n</ul>",
		"<ol>\n<li>first</li>",
		`<code class="language-go" data-lang="go">fmt.Println(&#34;&lt;hi&gt;&#34;)</code>`,
		"<blockquote>note</blockquote>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestMarkdownCodeSpansAreNotFormatted(t *testing.T) {
	got := string(Markdown("Use `a**b**c` here"))
	if !strings.Contains(got, "<code>a**b**c</code>") {
		t.Errorf("formatting applied inside code span: %s", got)
	}
}

func TestMarkdownUnclosedFenceAndBadLanguage(t *testing.T) {
	got := string(Markdown("```\" onmouseover=\"x\nbody"))
	if strings.Contains(got, "onmouseover") && !strings.Contains(got, "&#34;") {
		t.Errorf("language attribute injection: %s", got)
	}
	if !strings.Contains(got, "body") {
		t.Errorf("unclosed fence lost content: %s", got)
	}
}
