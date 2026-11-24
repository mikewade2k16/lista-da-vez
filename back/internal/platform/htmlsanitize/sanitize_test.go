package htmlsanitize

import (
	"strings"
	"testing"
)

func TestRichTextRemovesScriptsAndHandlers(t *testing.T) {
	cases := []string{
		`<p>oi<script>alert(1)</script></p>`,
		`<img src=x onerror="alert(1)">`,
		`<a href="javascript:alert(1)">x</a>`,
		`<iframe src="https://evil.example"></iframe>`,
		`<p style="background:url(javascript:alert(1))">x</p>`,
		`<svg><script>alert(1)</script></svg>`,
	}
	for _, input := range cases {
		out := strings.ToLower(RichText(input))
		for _, bad := range []string{"<script", "onerror", "javascript:", "<iframe", "style=", "<svg"} {
			if strings.Contains(out, bad) {
				t.Fatalf("%q sobreviveu em %q -> %q", bad, input, out)
			}
		}
	}
}

func TestRichTextKeepsFormatting(t *testing.T) {
	input := `<p><strong>negrito</strong> <em>it</em> <a href="https://omni.example/x">link</a></p><ul><li>a</li></ul><span class="mention" data-id="u1">@ana</span>`
	out := RichText(input)
	for _, keep := range []string{"<strong>negrito</strong>", "<em>it</em>", `href="https://omni.example/x"`, "<li>a</li>", `data-id="u1"`, `class="mention"`} {
		if !strings.Contains(out, keep) {
			t.Fatalf("perdeu %q: %q", keep, out)
		}
	}
	if RichText("") != "" {
		t.Fatal("vazio deve continuar vazio")
	}
}
