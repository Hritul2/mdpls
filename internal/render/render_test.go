package render

import (
	"strings"
	"testing"

	"github.com/Hritul2/mdpls/internal/config"
)

func render(t *testing.T, allowHTML bool, src string) string {
	t.Helper()
	out, err := New(allowHTML).Markdown([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestGitHubFlavoredFeatures(t *testing.T) {
	out := render(t, true, "| a | b |\n|---|---|\n| 1 | 2 |\n\n- [x] done\n- [ ] todo\n\n~~gone~~\n\nhttps://example.com\n")
	for _, want := range []string{"<table>", `type="checkbox"`, "checked", "<del>gone</del>", `<a href="https://example.com">`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHeadingIDsMatchGitHubStyle(t *testing.T) {
	out := render(t, true, "## Second Part\n")
	if !strings.Contains(out, `id="second-part"`) {
		t.Fatalf("heading id not GitHub-style:\n%s", out)
	}
}

func TestFrontMatterIsHidden(t *testing.T) {
	out := render(t, true, "---\ntitle: Secret Title\ntags: [a]\n---\n# Visible\n")
	if strings.Contains(out, "Secret Title") {
		t.Fatalf("front matter leaked:\n%s", out)
	}
	if !strings.Contains(out, "Visible") {
		t.Fatalf("body missing:\n%s", out)
	}
}

func TestStripFrontMatter(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"none":          {"# Hi\n", "# Hi\n"},
		"yaml":          {"---\na: 1\n---\nbody\n", "body\n"},
		"crlf":          {"---\r\na: 1\r\n---\r\nbody\r\n", "body\r\n"},
		"unterminated":  {"---\nnot closed\n", "---\nnot closed\n"},
		"hr not at top": {"text\n---\nmore\n", "text\n---\nmore\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := string(StripFrontMatter([]byte(c.in))); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestCodeIsHighlightedWithClasses(t *testing.T) {
	out := render(t, true, "```go\nfunc main() {}\n```\n")
	if !strings.Contains(out, `class="chroma"`) || !strings.Contains(out, `<span class="kd">func</span>`) {
		t.Fatalf("code not highlighted with chroma classes:\n%s", out)
	}
}

func TestRawHTMLToggle(t *testing.T) {
	src := "<details><summary>More</summary>hidden</details>\n"
	if out := render(t, true, src); !strings.Contains(out, "<details>") {
		t.Fatalf("allow_html=true should keep raw HTML:\n%s", out)
	}
	if out := render(t, false, src); strings.Contains(out, "<details>") {
		t.Fatalf("allow_html=false should drop raw HTML:\n%s", out)
	}
}

func TestCSSUsesThemeAndCodeStyle(t *testing.T) {
	th := config.Default().Theme
	th.Accent = "#123456"
	css, err := CSS(th)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#123456", th.Background, ".chroma"} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS missing %q", want)
		}
	}
	th.CodeStyle = "dracula"
	other, err := CSS(th)
	if err != nil {
		t.Fatal(err)
	}
	if other == css {
		t.Fatal("changing code_style should change the CSS")
	}
}

func TestPageEscapesTitleAndEmbedsBody(t *testing.T) {
	page, err := Page(Doc{Title: "<x>", Body: []byte("<p>hello</p>"), Live: true})
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, "<title>&lt;x&gt;</title>") {
		t.Errorf("title not escaped:\n%s", s)
	}
	if !strings.Contains(s, "<p>hello</p>") || !strings.Contains(s, "EventSource") {
		t.Errorf("body or live-reload script missing:\n%s", s)
	}
}

func TestCSSColorScheme(t *testing.T) {
	th := config.Default().Theme
	th.ColorScheme = "light"
	css, err := CSS(th)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(css, "color-scheme: light") || !strings.Contains(css, "tab-size: 4") {
		t.Fatal("CSS should follow theme.color_scheme and set tab-size")
	}
}

func TestIndexPageKeepsFoldersOpenAcrossRefresh(t *testing.T) {
	page, err := Page(Doc{Title: "x", Body: []byte("<p>x</p>"), Live: true, Index: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "details[data-dir]") {
		t.Fatal("index page script should restore open folders")
	}
}
