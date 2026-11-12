package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hritul2/mdpls/internal/config"
)

// repo builds a small docs tree and returns its root.
func repo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"README.md":           "# Readme\n\nSee [guide](docs/guide.md).\n",
		"docs/guide.md":       "---\ntitle: hidden\n---\n# Guide\n\n## Second Part\n",
		"docs/my notes.md":    "# Notes\n",
		"docs/api/ref.md":     "# Ref\n",
		"docs/img.png":        "\x89PNG\r\n\x1a\nfake",
		"node_modules/x/a.md": "# should be ignored\n",
		".git/HEAD":           "ref: x\n",
		".hidden/secret.md":   "# hidden dir\n",
		"main.go":             "package main\n",
	}
	for p, body := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func newServer(t *testing.T, root string) *Server {
	t.Helper()
	s, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func get(t *testing.T, s *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = "127.0.0.1:1112"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestIndexListsMarkdownAndSkipsIgnored(t *testing.T) {
	rec := get(t, newServer(t, repo(t)), "/")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`href="/README.md"`, `href="/docs/guide.md"`, `href="/docs/my%20notes.md"`} {
		if !strings.Contains(body, want) {
			t.Errorf("index missing %s", want)
		}
	}
	for _, bad := range []string{"node_modules", ".hidden", "main.go"} {
		if strings.Contains(body, bad) {
			t.Errorf("index should not list %s", bad)
		}
	}
}

func TestRendersMarkdownPage(t *testing.T) {
	rec := get(t, newServer(t, repo(t)), "/docs/guide.md")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	for _, want := range []string{`id="second-part"`, "/__mdpls/style.css", "EventSource"} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(body, "title: hidden") {
		t.Error("front matter leaked")
	}
}

func TestSpacesInNames(t *testing.T) {
	if rec := get(t, newServer(t, repo(t)), "/docs/my%20notes.md"); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPartialIsFragmentOnly(t *testing.T) {
	rec := get(t, newServer(t, repo(t)), "/docs/guide.md?partial=1")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("partial should be a bare fragment, got %d:\n%s", rec.Code, rec.Body.String())
	}
}

func TestDirectoryRedirectsThenListsOnlyItsFiles(t *testing.T) {
	s := newServer(t, repo(t))
	rec := get(t, s, "/docs")
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/docs/" {
		t.Fatalf("got %d Location=%q", rec.Code, rec.Header().Get("Location"))
	}
	body := get(t, s, "/docs/").Body.String()
	if !strings.Contains(body, "/docs/guide.md") || strings.Contains(body, "README.md") {
		t.Fatalf("directory listing not scoped:\n%s", body)
	}
}

func TestServesOtherFilesRaw(t *testing.T) {
	rec := get(t, newServer(t, repo(t)), "/docs/img.png")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestStylesheet(t *testing.T) {
	rec := get(t, newServer(t, repo(t)), "/__mdpls/style.css")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), config.Default().Theme.Accent) {
		t.Fatal("stylesheet missing theme accent")
	}
}

func TestCannotEscapeRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	targets := []string{"/../secret.txt", "/%2e%2e/secret.txt"}
	// Creating symlinks needs extra privileges on Windows; test it where we can.
	if err := os.Symlink(filepath.Join(parent, "secret.txt"), filepath.Join(root, "link.txt")); err == nil {
		targets = append(targets, "/link.txt")
	}
	s := newServer(t, root)
	for _, target := range targets {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = target // bypass client-side cleaning
		req.URL.RawPath = ""
		req.Host = "127.0.0.1"
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "TOP SECRET") {
			t.Fatalf("%s leaked a file outside the root (status %d)", target, rec.Code)
		}
	}
}

func TestRejectsForeignHost(t *testing.T) {
	s := newServer(t, repo(t))
	for host, want := range map[string]int{"evil.example.com": 403, "localhost:1112": 200, "127.0.0.1": 200, "[::1]:1112": 200} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q: status %d, want %d", host, rec.Code, want)
		}
	}
}

func TestOnlyGetAndHead(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/README.md", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	newServer(t, repo(t)).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	if rec := get(t, newServer(t, repo(t)), "/nope.md"); rec.Code != 404 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestLiveReloadEvents(t *testing.T) {
	root := repo(t)
	s := newServer(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.watch = watchTiming{poll: 50 * time.Millisecond, rescan: 200 * time.Millisecond}
	go s.Watch(ctx)

	ts := httptest.NewServer(s)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/__mdpls/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	events := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data: ") {
				events <- strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	time.Sleep(200 * time.Millisecond) // let the watcher settle

	expect := func(want string, act func()) {
		t.Helper()
		act()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case got := <-events:
				if got == want {
					return
				}
			case <-deadline:
				t.Fatalf("no event %q", want)
			}
		}
	}
	expect("/docs/guide.md", func() {
		os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("# Changed\n"), 0o644)
	})
	// A directory created after startup is watched too.
	expect("/new/fresh.md", func() {
		os.MkdirAll(filepath.Join(root, "new"), 0o755)
		time.Sleep(300 * time.Millisecond)
		os.WriteFile(filepath.Join(root, "new", "fresh.md"), []byte("# Fresh\n"), 0o644)
	})
}

func TestIndexIsANestedTree(t *testing.T) {
	body := get(t, newServer(t, repo(t)), "/").Body.String()
	docs := strings.Index(body, `<details class="mdpls-dir" data-dir="docs">`)
	api := strings.Index(body, `<details class="mdpls-dir" data-dir="docs/api">`)
	ref := strings.Index(body, `href="/docs/api/ref.md"`)
	guide := strings.Index(body, `href="/docs/guide.md"`)
	readme := strings.Index(body, `href="/README.md"`)
	if docs < 0 || api < 0 {
		t.Fatalf("folders not rendered as <details>:\n%s", body)
	}
	// docs/ contains docs/api/, which contains ref.md; folders come before files.
	if !(docs < api && api < ref && ref < guide) {
		t.Fatalf("wrong nesting/order: docs=%d api=%d ref=%d guide=%d\n%s", docs, api, ref, guide, body)
	}
	// Root files sit outside every folder, after them.
	if readme < strings.LastIndex(body, "</details>") {
		t.Fatalf("README.md should be a root entry after the folders:\n%s", body)
	}
	// Each folder shows how many markdown files it holds (recursively).
	if !strings.Contains(body, `docs</span><span class="mdpls-count">3</span>`) {
		t.Fatalf("docs folder count missing or wrong:\n%s", body)
	}
}
