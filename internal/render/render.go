// Package render turns markdown into themed HTML pages.
package render

import (
	"bytes"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Renderer converts GitHub-flavored markdown to HTML fragments.
type Renderer struct {
	md goldmark.Markdown
}

// New builds a renderer. allowHTML keeps raw HTML from the markdown source
// (like GitHub's <details>, <img>, <br>); when false it is dropped.
func New(allowHTML bool) *Renderer {
	rendererOpts := []goldmark.Option{}
	if allowHTML {
		rendererOpts = append(rendererOpts, goldmark.WithRendererOptions(html.WithUnsafe()))
	}
	md := goldmark.New(append(rendererOpts,
		goldmark.WithExtensions(
			extension.GFM, // tables, task lists, strikethrough, autolinks
			extension.Footnote,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)...)
	return &Renderer{md: md}
}

// Markdown renders src (front matter removed) to an HTML fragment.
func (r *Renderer) Markdown(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(StripFrontMatter(src), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// StripFrontMatter removes a leading YAML front matter block (--- ... ---).
// Anything else, including an unterminated block, is returned unchanged.
func StripFrontMatter(src []byte) []byte {
	nl := []byte("\n")
	if bytes.HasPrefix(src, []byte("---\r\n")) {
		nl = []byte("\r\n")
	} else if !bytes.HasPrefix(src, []byte("---\n")) {
		return src
	}
	rest := src[3+len(nl):]
	closing := append(append([]byte{}, nl...), []byte("---")...)
	// The block may be empty: the closing fence can be the very next line.
	if bytes.HasPrefix(rest, []byte("---")) {
		return trimFence(rest[3:], nl, src)
	}
	i := bytes.Index(rest, closing)
	if i < 0 {
		return src
	}
	return trimFence(rest[i+len(closing):], nl, src)
}

// trimFence drops the newline that ends the closing fence line. If the fence
// line has trailing text it isn't a fence, so the original is returned.
func trimFence(after, nl, orig []byte) []byte {
	if len(after) == 0 {
		return after
	}
	if bytes.HasPrefix(after, nl) {
		return after[len(nl):]
	}
	return orig
}
