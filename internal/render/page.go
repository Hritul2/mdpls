package render

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	texttemplate "text/template"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/Hritul2/mdpls/internal/config"
)

// Doc is one page to render.
type Doc struct {
	Title string
	Body  []byte // trusted HTML fragment (from Markdown or the index)
	Live  bool   // include the live-reload script
	Index bool   // a directory listing: refresh on any markdown change
}

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link rel="stylesheet" href="/__mdpls/style.css">
</head><body><main id="page"><article class="markdown-body" id="md">{{.Body}}</article></main>
{{if .Live}}<script>
(function () {
  var base = document.title, index = {{.Index}};
  var es = new EventSource('/__mdpls/events');
  es.onopen = function () { document.title = base; };
  es.onerror = function () { document.title = '⏸ ' + base; };
  es.onmessage = function (e) {
    if (!index && e.data !== decodeURIComponent(location.pathname)) return;
    fetch(location.pathname + '?partial=1', { cache: 'no-store' })
      .then(function (r) { return r.ok ? r.text() : null; })
      .then(function (h) { if (h !== null) document.getElementById('md').innerHTML = h; });
  };
})();
</script>{{end}}
</body></html>
`))

// Page wraps a fragment in a full HTML document.
func Page(d Doc) ([]byte, error) {
	var buf bytes.Buffer
	err := pageTmpl.Execute(&buf, struct {
		Title string
		Body  template.HTML
		Live  bool
		Index bool
	}{d.Title, template.HTML(d.Body), d.Live, d.Index})
	return buf.Bytes(), err
}

var cssTmpl = texttemplate.Must(texttemplate.New("css").Parse(`:root {
  color-scheme: {{.ColorScheme}};
  --bg: {{.Background}};
  --surface: {{.Surface}};
  --text: {{.Text}};
  --muted: {{.Muted}};
  --border: {{.Border}};
  --accent: {{.Accent}};
  --radius: {{.Radius}};
  --font-body: {{.FontBody}};
  --font-mono: {{.FontMono}};
}
`))

// baseCSS is the theme-independent part; it only reads the variables above.
const baseCSS = `
html, body { margin: 0; min-height: 100vh; background: var(--bg); }
main { max-width: var(--max-width); margin: 0 auto; padding: 3rem 1rem 6rem; }
::selection { background: var(--accent); color: var(--bg); }

.markdown-body {
  font-family: var(--font-body); font-size: var(--font-size); line-height: var(--line-height);
  color: var(--text); word-wrap: break-word;
  -webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale;
}
.markdown-body * { box-sizing: border-box; }
.markdown-body > :first-child { margin-top: 0; }
.markdown-body > :last-child { margin-bottom: 0; }

.markdown-body h1, .markdown-body h2, .markdown-body h3,
.markdown-body h4, .markdown-body h5, .markdown-body h6 {
  color: var(--text); font-weight: 600; letter-spacing: -0.02em; line-height: 1.25;
}
.markdown-body h1 { font-size: 2em; margin: 2rem 0 1rem; }
.markdown-body h2 { font-size: 1.5em; margin: 1.75rem 0 0.75rem; }
.markdown-body h3 { font-size: 1.25em; margin: 1.5rem 0 0.5rem; }
.markdown-body h4, .markdown-body h5, .markdown-body h6 { font-size: 1em; margin: 1.25rem 0 0.5rem; }
.markdown-body h6 { color: var(--muted); }
.markdown-body p { margin: 1rem 0; }

.markdown-body strong { color: var(--accent); font-weight: 700; }
.markdown-body a { color: var(--accent); text-decoration: underline; text-underline-offset: 3px; }
.markdown-body a:hover { opacity: 0.8; }

.markdown-body code {
  font-family: var(--font-mono); font-size: 0.85em;
  background: color-mix(in srgb, var(--text) 8%, transparent);
  border-radius: 4px; padding: 0.15em 0.4em; overflow-wrap: break-word;
}
.markdown-body pre {
  margin: 1.25rem 0; padding: 1.25rem; overflow-x: auto;
  font-size: 0.85em; line-height: 1.6; tab-size: 4;
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--text) 10%, transparent);
  border-radius: var(--radius);
}
.markdown-body pre code { background: transparent; padding: 0; font-size: inherit; overflow-wrap: normal; }

.markdown-body blockquote {
  margin: 1.25rem 0; padding: 0 0 0 1rem;
  border-left: 3px solid color-mix(in srgb, var(--accent) 35%, transparent);
  color: color-mix(in srgb, var(--text) 60%, transparent); font-style: italic;
}
.markdown-body ul, .markdown-body ol { padding-left: 1.5rem; margin: 1rem 0; }
.markdown-body li { margin: 0.35rem 0; }
.markdown-body li::marker { color: color-mix(in srgb, var(--text) 50%, transparent); }
.markdown-body li:has(> input[type="checkbox"]) { list-style: none; }
.markdown-body li > input[type="checkbox"] { margin: 0 0.5em 0.2em -1.5rem; vertical-align: middle; accent-color: var(--accent); }

.markdown-body table {
  display: table; width: 100%; margin: 1.25rem 0; font-size: 0.875em;
  border-collapse: separate; border-spacing: 0;
}
.markdown-body th, .markdown-body td {
  padding: 0.75rem 1.25rem; text-align: left; vertical-align: top;
  border-bottom: 1px solid color-mix(in srgb, var(--text) 8%, transparent);
}
.markdown-body th:first-child, .markdown-body td:first-child { border-left: 1px solid color-mix(in srgb, var(--text) 10%, transparent); }
.markdown-body th:last-child, .markdown-body td:last-child { border-right: 1px solid color-mix(in srgb, var(--text) 10%, transparent); }
.markdown-body th {
  font-weight: 500; background: color-mix(in srgb, var(--text) 4%, transparent);
  border-top: 1px solid color-mix(in srgb, var(--text) 10%, transparent);
}
.markdown-body th:first-child { border-top-left-radius: var(--radius); }
.markdown-body th:last-child { border-top-right-radius: var(--radius); }
.markdown-body tr:last-child td:first-child { border-bottom-left-radius: var(--radius); }
.markdown-body tr:last-child td:last-child { border-bottom-right-radius: var(--radius); }

/* Horizontal rule: a hollow node and a line */
.markdown-body hr {
  height: 10px; margin: 2rem 0; border: 0;
  background:
    radial-gradient(circle closest-side, transparent 3.6px,
      color-mix(in srgb, var(--text) 25%, transparent) 4.2px,
      color-mix(in srgb, var(--text) 25%, transparent) 4.8px, transparent 5px)
      left center / 10px 10px no-repeat,
    linear-gradient(var(--border), var(--border)) 26px 50% / calc(100% - 26px) 1px no-repeat;
}
.markdown-body img { max-width: 100%; border-radius: 8px; }
.markdown-body kbd {
  font: 0.75em var(--font-mono); padding: 3px 6px; border-radius: 4px;
  background: var(--surface); border: 1px solid var(--border);
  box-shadow: inset 0 -1px 0 var(--border);
}
.markdown-body details { margin: 1rem 0; }
.markdown-body summary { cursor: pointer; }
.markdown-body .footnotes { font-size: 0.875em; color: var(--muted); }

/* Index pages */
.mdpls-meta { color: var(--muted); }
`

// CSS builds the full stylesheet for a theme: variables, layout, and the
// chroma code style.
func CSS(t config.Theme) (string, error) {
	var b strings.Builder
	if err := cssTmpl.Execute(&b, t); err != nil {
		return "", err
	}
	fmt.Fprintf(&b, ":root { --font-size: %s; --line-height: %s; --max-width: %s; }\n", t.FontSize, t.LineHeight, t.MaxWidth)
	b.WriteString(baseCSS)

	style := styles.Get(t.CodeStyle)
	if err := chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&b, style); err != nil {
		return "", err
	}
	// Keep code blocks on the theme's surface so they match the page.
	b.WriteString("\n.markdown-body pre.chroma { background: var(--surface); }\n")
	return b.String(), nil
}
