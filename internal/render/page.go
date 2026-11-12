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
  var md = document.getElementById('md');

  // Index pages: remember which folders are open (per browser), so live
  // refreshes and reloads don't collapse them.
  var KEY = 'mdpls:open-folders';
  function load() { try { return JSON.parse(localStorage.getItem(KEY) || '[]'); } catch (e) { return []; } }
  function save() {
    var open = load().filter(function (p) { return !md.querySelector('details[data-dir="' + CSS.escape(p) + '"]'); });
    md.querySelectorAll('details[data-dir][open]').forEach(function (d) { open.push(d.dataset.dir); });
    try { localStorage.setItem(KEY, JSON.stringify(open)); } catch (e) {}
  }
  function restore() {
    load().forEach(function (p) {
      var d = md.querySelector('details[data-dir="' + CSS.escape(p) + '"]');
      if (d) d.open = true;
    });
  }
  function setAll(open) {
    md.querySelectorAll('details[data-dir]').forEach(function (d) { d.open = open; });
    save();
  }
  function controls() {
    var meta = md.querySelector('.mdpls-meta');
    if (!meta || !md.querySelector('details[data-dir]') || meta.querySelector('button')) return;
    var btn = document.createElement('button');
    btn.type = 'button'; btn.className = 'mdpls-btn';
    function sync() {
      var all = !md.querySelector('details[data-dir]:not([open])');
      btn.textContent = all ? 'Collapse all' : 'Expand all';
      btn.dataset.open = all ? '1' : '';
    }
    btn.onclick = function () { setAll(!btn.dataset.open); sync(); };
    meta.appendChild(btn); sync();
    md.addEventListener('toggle', sync, true);
  }
  if (index) {
    restore(); controls();
    document.addEventListener('toggle', function (e) { if (e.target.matches && e.target.matches('details[data-dir]')) save(); }, true);
  }

  var es = new EventSource('/__mdpls/events');
  es.onopen = function () { document.title = base; };
  es.onerror = function () { document.title = '\u23F8 ' + base; };
  es.onmessage = function (e) {
    if (!index && e.data !== decodeURIComponent(location.pathname)) return;
    fetch(location.pathname + '?partial=1', { cache: 'no-store' })
      .then(function (r) { return r.ok ? r.text() : null; })
      .then(function (h) {
        if (h === null) return;
        md.innerHTML = h;
        if (index) { restore(); controls(); }
      });
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

/* Index pages: a collapsible folder tree */
.markdown-body h1.mdpls-title { position: relative; display: inline-block; cursor: default; }
.markdown-body h1.mdpls-title::after {
  content: attr(data-path); position: absolute; left: 0; top: 100%; margin-top: 0.25rem;
  font: 400 0.8rem var(--font-mono); color: var(--text); background: var(--surface);
  border: 1px solid var(--border); border-radius: 6px; padding: 0.3em 0.6em;
  white-space: nowrap; pointer-events: none; z-index: 10;
  opacity: 0; filter: blur(4px); transform: translateY(-3px);
  transition: opacity 100ms ease-out, filter 100ms ease-out, transform 100ms ease-out;
}
.markdown-body h1.mdpls-title:hover::after {
  opacity: 1; filter: blur(0); transform: none;
  transition-duration: 160ms;
}
@media (prefers-reduced-motion: reduce) { .markdown-body h1.mdpls-title::after { transition: none; filter: none; transform: none; } }
.mdpls-meta { color: var(--muted); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.75rem; font-size: 0.875em; padding-bottom: 0.75rem; margin-bottom: 0.75rem; border-bottom: 1px solid var(--border); }
.mdpls-btn {
  font: inherit; color: var(--accent); background: transparent; border: 0;
  padding: 0.1em 0.25em; cursor: pointer; border-radius: 4px;
}
.mdpls-btn:hover { text-decoration: underline; }
.markdown-body ul.mdpls-tree { list-style: none; margin: 0; padding-left: 0; }
.markdown-body ul.mdpls-tree ul.mdpls-tree {
  margin-left: 0.55rem; padding-left: 1rem;
  border-left: 1px solid var(--border);
}
.markdown-body ul.mdpls-tree li { margin: 0.1rem 0; }
.markdown-body .mdpls-dir { margin: 0; }
.markdown-body .mdpls-dir > summary {
  list-style: none; cursor: pointer; display: flex; align-items: center; gap: 0.5rem;
  padding: 0.2rem 0; color: var(--text); font-weight: 500; user-select: none;
}
.markdown-body .mdpls-dir > summary::-webkit-details-marker { display: none; }
.markdown-body .mdpls-dir > summary::before {
  content: ""; width: 0.4rem; height: 0.4rem; flex: none;
  border-right: 1.5px solid var(--muted); border-bottom: 1.5px solid var(--muted);
  transform: rotate(-45deg); transition: transform 120ms ease; margin: 0 0.15rem;
}
.markdown-body .mdpls-dir[open] > summary::before { transform: rotate(45deg); }
.markdown-body .mdpls-dir > summary:hover .mdpls-name { color: var(--accent); }
.markdown-body .mdpls-count {
  font-size: 0.75em; color: var(--muted); font-weight: 400;
  border: 1px solid var(--border); border-radius: 999px; padding: 0 0.5em; line-height: 1.5;
}
.markdown-body .mdpls-file { padding: 0.15rem 0 0.15rem 1.1rem; }
.markdown-body .mdpls-file a { text-decoration: none; color: var(--accent); }
.markdown-body .mdpls-file a:hover { color: var(--accent); text-decoration: underline; }
@media (prefers-reduced-motion: reduce) { .markdown-body .mdpls-dir > summary::before { transition: none; } }
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
