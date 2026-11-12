package app

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer lets the test read output while Run is still writing it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func env(t *testing.T) (*App, *syncBuffer, *syncBuffer, *[]string) {
	t.Helper()
	t.Setenv("MDPLS_HOME", t.TempDir())
	out, errOut := &syncBuffer{}, &syncBuffer{}
	var opened []string
	a := &App{Stdout: out, Stderr: errOut, Version: "1.2.3", Open: func(u string) { opened = append(opened, u) }}
	return a, out, errOut, &opened
}

func TestHelpAndVersion(t *testing.T) {
	a, out, _, _ := env(t)
	if code := a.Run(context.Background(), []string{"--help"}); code != 0 || !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help: code %d out %q", code, out.String())
	}
	a, out, _, _ = env(t)
	if code := a.Run(context.Background(), []string{"--version"}); code != 0 || strings.TrimSpace(out.String()) != "mdpls 1.2.3" {
		t.Fatalf("version: code %d out %q", code, out.String())
	}
}

func TestWrongUsagePointsToHelp(t *testing.T) {
	for _, args := range [][]string{{"--halp"}, {"help"}, {"a", "b"}} {
		a, _, errOut, _ := env(t)
		code := a.Run(context.Background(), args)
		if code != 2 || !strings.Contains(errOut.String(), "mdpls --help") {
			t.Errorf("%q: code %d stderr %q", args, code, errOut.String())
		}
	}
}

func TestConfigCommandCreatesAndPrints(t *testing.T) {
	a, out, _, _ := env(t)
	if code := a.Run(context.Background(), []string{"config"}); code != 0 {
		t.Fatalf("code %d", code)
	}
	path := filepath.Join(os.Getenv("MDPLS_HOME"), "config.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config not created: %v", err)
	}
	if !strings.Contains(out.String(), path) || !strings.Contains(out.String(), `"port": 1112`) {
		t.Fatalf("output %q", out.String())
	}
}

func TestBadConfigIsReported(t *testing.T) {
	a, _, errOut, _ := env(t)
	os.WriteFile(filepath.Join(os.Getenv("MDPLS_HOME"), "config.json"), []byte("{oops"), 0o644)
	if code := a.Run(context.Background(), []string{"config"}); code != 1 || !strings.Contains(errOut.String(), "not valid JSON") {
		t.Fatalf("code %d stderr %q", code, errOut.String())
	}
}

func freePort(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, p, _ := net.SplitHostPort(l.Addr().String())
	return p
}

func TestServeOpensFileAndStopsOnCancel(t *testing.T) {
	a, out, errOut, opened := env(t)
	repo := t.TempDir()
	os.Mkdir(filepath.Join(repo, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repo, "docs"), 0o755)
	os.WriteFile(filepath.Join(repo, "docs", "a.md"), []byte("# Hello mdpls\n"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int)
	port := freePort(t)
	go func() { done <- a.Run(ctx, []string{filepath.Join(repo, "docs", "a.md"), "--port", port}) }()

	url := waitForURL(t, out)
	if !strings.HasSuffix(url, ":"+port+"/docs/a.md") {
		t.Fatalf("banner URL %q, want the file on port %s", url, port)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Hello mdpls") {
		t.Fatalf("page body %q", body)
	}
	if len(*opened) != 1 || (*opened)[0] != url {
		t.Fatalf("browser opened %v, want [%s]", *opened, url)
	}

	cancel() // what Ctrl-C does
	select {
	case code := <-done:
		if code != 0 || !strings.Contains(out.String(), "stopped") {
			t.Fatalf("code %d out %q stderr %q", code, out.String(), errOut.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestNoOpenAndRootIndexForDirectory(t *testing.T) {
	a, out, _, opened := env(t)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx, []string{dir, "--no-open", "--port", freePort(t)})
	url := waitForURL(t, out)
	if !strings.HasSuffix(url, "/") {
		t.Fatalf("directory should open the index, got %q", url)
	}
	time.Sleep(100 * time.Millisecond)
	if len(*opened) != 0 {
		t.Fatalf("--no-open still opened %v", *opened)
	}
}

func TestMissingPath(t *testing.T) {
	a, _, errOut, _ := env(t)
	if code := a.Run(context.Background(), []string{"/no/such/file.md"}); code != 2 || !strings.Contains(errOut.String(), "no such file or directory") {
		t.Fatalf("code %d stderr %q", code, errOut.String())
	}
}

var urlRe = regexp.MustCompile(`http://127\.0\.0\.1:\d+/\S*`)

func waitForURL(t *testing.T, out *syncBuffer) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m := urlRe.FindString(out.String()); m != "" {
			return m
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no URL printed; output %q", out.String())
	return ""
}
