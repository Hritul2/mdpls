// Package app is the mdpls command: argument handling, startup and shutdown.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Hritul2/mdpls/internal/cli"
	"github.com/Hritul2/mdpls/internal/config"
	"github.com/Hritul2/mdpls/internal/port"
	"github.com/Hritul2/mdpls/internal/root"
	"github.com/Hritul2/mdpls/internal/server"
)

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// portTries is how many ports after the configured one are tried.
const portTries = 50

// App holds the command's dependencies so tests can replace them.
type App struct {
	Stdout, Stderr io.Writer
	Version        string
	Open           func(url string) // opens a URL in the browser
}

func (a *App) usageError(msg string) int {
	fmt.Fprintf(a.Stderr, "mdpls: %s\n       Run `mdpls --help` to see how to use it.\n", msg)
	return ExitUsage
}

func (a *App) fail(err error) int {
	fmt.Fprintf(a.Stderr, "mdpls: %v\n", err)
	return ExitError
}

// Run executes mdpls with args (without the program name). It returns when
// the command finishes or, for the server, when ctx is cancelled (Ctrl-C).
func (a *App) Run(ctx context.Context, args []string) int {
	opts, err := cli.Parse(args)
	if err != nil {
		return a.usageError(err.Error())
	}
	switch opts.Command {
	case cli.CmdHelp:
		fmt.Fprintln(a.Stdout, cli.Usage)
		return ExitOK
	case cli.CmdVersion:
		fmt.Fprintf(a.Stdout, "mdpls %s\n", a.Version)
		return ExitOK
	}

	dir, err := config.Dir()
	if err != nil {
		return a.fail(err)
	}
	cfg, _, err := config.Load(dir)
	if err != nil {
		return a.fail(err)
	}
	cfgPath := filepath.Join(dir, config.FileName)

	if opts.Command == cli.CmdConfig {
		raw, err := os.ReadFile(cfgPath)
		if err != nil {
			return a.fail(err)
		}
		fmt.Fprintf(a.Stdout, "%s\n\n%s", cfgPath, raw)
		return ExitOK
	}
	return a.serve(ctx, opts, cfg, cfgPath)
}

func (a *App) serve(ctx context.Context, opts cli.Options, cfg config.Config, cfgPath string) int {
	target := opts.Path
	if target == "" {
		target = "."
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return a.fail(err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return a.usageError("no such file or directory: " + opts.Path)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	start := abs
	if !info.IsDir() {
		start = filepath.Dir(abs)
	}

	srv, err := server.New(root.Find(start), cfg)
	if err != nil {
		return a.fail(err)
	}

	first := cfg.Port
	if opts.Port != 0 {
		first = opts.Port
	}
	ln, err := port.Listen(first, portTries)
	if err != nil {
		return a.fail(err)
	}
	bound := ln.Addr().(*net.TCPAddr).Port
	base := fmt.Sprintf("http://127.0.0.1:%d", bound)
	link := base + targetPath(srv.Root(), abs, info.IsDir())

	fmt.Fprintf(a.Stdout, "mdpls  serving %s\n", srv.Root())
	fmt.Fprintf(a.Stdout, "       %s\n", link)
	if bound != first {
		fmt.Fprintf(a.Stdout, "       (port %d was taken)\n", first)
	}
	fmt.Fprintf(a.Stdout, "       config %s\n", cfgPath)
	fmt.Fprintln(a.Stdout, "       Ctrl-C to stop")
	if cfg.OpenBrowser && !opts.NoOpen && a.Open != nil {
		a.Open(link)
	}

	// Request contexts derive from ctx, so Ctrl-C also ends the long-lived
	// live-reload streams; otherwise Shutdown would wait on them forever.
	httpSrv := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go srv.Watch(ctx)
	errc := make(chan error, 1)
	go func() { errc <- httpSrv.Serve(ln) }()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return a.fail(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			httpSrv.Close()
		}
	}
	fmt.Fprintln(a.Stdout, "\nmdpls: stopped")
	return ExitOK
}

// targetPath is the URL path for what the user pointed at: a markdown file,
// a directory listing, or the root index for anything else.
func targetPath(rootDir, abs string, isDir bool) string {
	rel, err := filepath.Rel(rootDir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "/"
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "/"
	}
	escaped := (&url.URL{Path: "/" + rel}).EscapedPath()
	if isDir {
		return escaped + "/"
	}
	if server.IsMarkdown(abs) {
		return escaped
	}
	return "/"
}
