// Package server serves a directory's markdown as themed, live-reloading HTML.
package server

import (
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Hritul2/mdpls/internal/config"
	"github.com/Hritul2/mdpls/internal/render"
)

// MaxIndexFiles caps directory listings so huge trees stay responsive.
const MaxIndexFiles = 5000

// Server serves one root directory. It is an http.Handler.
type Server struct {
	root   string // symlink-resolved absolute root
	cfg    config.Config
	md     *render.Renderer
	css    []byte
	ignore map[string]bool
	hub    *hub
	watch  watchTiming
}

// New prepares a server for root.
func New(root string, cfg config.Config) (*Server, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	css, err := render.CSS(cfg.Theme)
	if err != nil {
		return nil, err
	}
	ignore := map[string]bool{}
	for _, n := range cfg.Ignore {
		ignore[n] = true
	}
	return &Server{
		root:   real,
		cfg:    cfg,
		md:     render.New(cfg.AllowHTML),
		css:    []byte(css),
		ignore: ignore,
		hub:    newHub(),
		watch:  defaultTiming,
	}, nil
}

// Root is the directory being served.
func (s *Server) Root() string { return s.root }

// IsMarkdown reports whether a file name has a markdown extension.
func IsMarkdown(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

func allowedHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// ServeHTTP routes a request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only answer requests addressed to loopback names: this defeats DNS
	// rebinding, where a web page points its own hostname at 127.0.0.1.
	if !allowedHost(r.Host) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/__mdpls/style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(s.css)
		return
	case "/__mdpls/events":
		s.events(w, r)
		return
	}

	// path.Clean on an absolute path can never climb above "/", so the join
	// stays inside root; symlinks are checked separately below.
	clean := path.Clean("/" + r.URL.Path)
	abs := filepath.Join(s.root, filepath.FromSlash(clean))
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		http.Error(w, "not found: "+clean, http.StatusNotFound)
		return
	}
	if real != s.root && !strings.HasPrefix(real, s.root+string(filepath.Separator)) {
		http.Error(w, "not found: "+clean, http.StatusNotFound)
		return
	}
	info, err := os.Stat(real)
	if err != nil {
		http.Error(w, "not found: "+clean, http.StatusNotFound)
		return
	}
	partial := r.URL.Query().Get("partial") == "1"

	switch {
	case info.IsDir():
		if clean != "/" && !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, (&url.URL{Path: clean + "/"}).EscapedPath(), http.StatusMovedPermanently)
			return
		}
		s.serveIndex(w, real, clean, partial)
	case IsMarkdown(real):
		s.serveMarkdown(w, real, partial)
	default:
		f, err := os.Open(real)
		if err != nil {
			http.Error(w, "not found: "+clean, http.StatusNotFound)
			return
		}
		defer f.Close()
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	}
}

func writeHTML(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	w.Write(body)
}

func (s *Server) page(w http.ResponseWriter, code int, d render.Doc, partial bool) {
	if partial {
		writeHTML(w, code, d.Body)
		return
	}
	out, err := render.Page(d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, code, out)
}

func (s *Server) serveMarkdown(w http.ResponseWriter, file string, partial bool) {
	title := filepath.Base(file)
	src, err := os.ReadFile(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err := s.md.Markdown(src)
	if err != nil {
		msg := fmt.Sprintf("<h1>Could not render %s</h1><pre>%s</pre>", html.EscapeString(title), html.EscapeString(err.Error()))
		s.page(w, http.StatusInternalServerError, render.Doc{Title: title, Body: []byte(msg), Live: true}, partial)
		return
	}
	s.page(w, http.StatusOK, render.Doc{Title: title, Body: body, Live: true}, partial)
}

// markdownFiles lists markdown files under dir as slash paths relative to root,
// skipping hidden and ignored directories.
func (s *Server) markdownFiles(dir string, limit int) []string {
	var out []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && (strings.HasPrefix(d.Name(), ".") || s.ignore[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		if IsMarkdown(d.Name()) {
			rel, _ := filepath.Rel(s.root, p)
			out = append(out, filepath.ToSlash(rel))
			if limit > 0 && len(out) >= limit {
				return filepath.SkipAll
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func escapePath(rel string) string {
	segs := strings.Split(rel, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return "/" + strings.Join(segs, "/")
}

func (s *Server) serveIndex(w http.ResponseWriter, dir, clean string, partial bool) {
	files := s.markdownFiles(dir, MaxIndexFiles)
	heading := filepath.Base(s.root)
	prefix := "" // the listed directory, relative to root, with a trailing slash
	if clean != "/" {
		heading = strings.TrimPrefix(clean, "/")
		prefix = heading + "/"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(heading))
	plural := "s"
	if len(files) == 1 {
		plural = ""
	}
	fmt.Fprintf(&b, `<p class="mdpls-meta">%d markdown file%s`, len(files), plural)
	if len(files) >= MaxIndexFiles {
		b.WriteString(" (list truncated)")
	}
	b.WriteString("</p>\n")

	tree := newDirNode("", strings.TrimSuffix(prefix, "/"))
	for _, f := range files {
		tree.add(f, strings.TrimPrefix(f, prefix))
	}
	tree.render(&b)
	s.page(w, http.StatusOK, render.Doc{Title: heading, Body: []byte(b.String()), Live: true, Index: true}, partial)
}

// dirNode is one folder in the index tree.
type dirNode struct {
	name  string // display name
	path  string // path from the root, e.g. "docs/api"
	dirs  map[string]*dirNode
	files []string // paths from the root
	count int      // markdown files in this folder and below
}

func newDirNode(name, p string) *dirNode {
	return &dirNode{name: name, path: p, dirs: map[string]*dirNode{}}
}

// add places file (a path from the root) using rel, its path below this node.
func (d *dirNode) add(file, rel string) {
	d.count++
	i := strings.Index(rel, "/")
	if i < 0 {
		d.files = append(d.files, file)
		return
	}
	name := rel[:i]
	child := d.dirs[name]
	if child == nil {
		childPath := name
		if d.path != "" {
			childPath = d.path + "/" + name
		}
		child = newDirNode(name, childPath)
		d.dirs[name] = child
	}
	child.add(file, rel[i+1:])
}

// render writes folders (collapsible, alphabetical) before files, like a file
// explorer.
func (d *dirNode) render(b *strings.Builder) {
	if len(d.dirs) == 0 && len(d.files) == 0 {
		return
	}
	b.WriteString(`<ul class="mdpls-tree">` + "\n")
	names := make([]string, 0, len(d.dirs))
	for n := range d.dirs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := d.dirs[n]
		fmt.Fprintf(b, `<li><details class="mdpls-dir" data-dir="%s"><summary><span class="mdpls-name">%s</span><span class="mdpls-count">%d</span></summary>`+"\n",
			html.EscapeString(c.path), html.EscapeString(c.name), c.count)
		c.render(b)
		b.WriteString("</details></li>\n")
	}
	for _, f := range d.files {
		fmt.Fprintf(b, `<li class="mdpls-file"><a href="%s">%s</a></li>`+"\n", escapePath(f), html.EscapeString(path.Base(f)))
	}
	b.WriteString("</ul>\n")
}
