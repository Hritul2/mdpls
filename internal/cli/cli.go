// Package cli parses mdpls command-line arguments.
package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// Command is what the user asked mdpls to do.
type Command int

const (
	CmdServe Command = iota
	CmdHelp
	CmdVersion
	CmdConfig
)

// Options is the parsed command line.
type Options struct {
	Command Command
	Path    string // file or directory to serve/open; "" means the current directory
	Port    int    // 0 means "use the configured port"
	NoOpen  bool
}

// Usage is printed for --help.
const Usage = `mdpls — preview a repo's markdown in the browser, live

Usage:
  mdpls [path] [options]
  mdpls config

  path           a markdown file (opened in the browser), a directory, or nothing
                 (the current directory). mdpls serves the git root of that
                 location, or the location itself outside a git repo.

Commands:
  config         print where the config file lives and what it contains

Options:
  --port <n>     first port to try (default from config: 1112). mdpls moves to
                 the next free port only if this one is already taken.
  --no-open      don't open the browser
  -v, --version  print the version
  -h, --help     show this help

Config:  ~/.mdpls/config.json (override the folder with $MDPLS_HOME)
Stop the server with Ctrl-C.`

// Parse turns raw arguments (without the program name) into Options.
// Flags may appear before or after the path.
func Parse(args []string) (Options, error) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return Options{Command: CmdHelp}, nil
		}
	}
	for _, a := range args {
		if a == "-v" || a == "--version" {
			return Options{Command: CmdVersion}, nil
		}
	}

	opts := Options{Command: CmdServe}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--no-open":
			opts.NoOpen = true
		case a == "--port" || strings.HasPrefix(a, "--port="):
			var v string
			if a == "--port" {
				if i+1 >= len(args) {
					return Options{}, fmt.Errorf("--port needs a value")
				}
				i++
				v = args[i]
			} else {
				v = strings.TrimPrefix(a, "--port=")
			}
			p, err := strconv.Atoi(v)
			if err != nil || p < 1 || p > 65535 {
				return Options{}, fmt.Errorf("invalid port: %s", v)
			}
			opts.Port = p
		case strings.HasPrefix(a, "-") && a != "-":
			return Options{}, fmt.Errorf("unknown option: %s", a)
		default:
			positional = append(positional, a)
		}
	}

	if len(positional) > 0 && positional[0] == "config" {
		if len(positional) > 1 {
			return Options{}, fmt.Errorf("config takes no arguments")
		}
		return Options{Command: CmdConfig}, nil
	}
	if len(positional) > 1 {
		return Options{}, fmt.Errorf("expected one path, got %d: %s", len(positional), strings.Join(positional, " "))
	}
	if len(positional) == 1 {
		opts.Path = positional[0]
	}
	return opts, nil
}
