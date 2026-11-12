package cli

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Options
		wantErr string
	}{
		{name: "no args", args: nil, want: Options{Command: CmdServe}},
		{name: "path", args: []string{"docs/a.md"}, want: Options{Command: CmdServe, Path: "docs/a.md"}},
		{name: "flags after path", args: []string{"a.md", "--port", "8080", "--no-open"}, want: Options{Command: CmdServe, Path: "a.md", Port: 8080, NoOpen: true}},
		{name: "port equals form", args: []string{"--port=9000"}, want: Options{Command: CmdServe, Port: 9000}},
		{name: "help long", args: []string{"--help"}, want: Options{Command: CmdHelp}},
		{name: "help short wins over others", args: []string{"a.md", "-h"}, want: Options{Command: CmdHelp}},
		{name: "version", args: []string{"--version"}, want: Options{Command: CmdVersion}},
		{name: "version short", args: []string{"-v"}, want: Options{Command: CmdVersion}},
		{name: "config subcommand", args: []string{"config"}, want: Options{Command: CmdConfig}},
		{name: "unknown flag", args: []string{"--halp"}, wantErr: "unknown option: --halp"},
		{name: "port missing value", args: []string{"--port"}, wantErr: "--port needs a value"},
		{name: "port not a number", args: []string{"--port", "abc"}, wantErr: "invalid port: abc"},
		{name: "port out of range", args: []string{"--port", "70000"}, wantErr: "invalid port: 70000"},
		{name: "two paths", args: []string{"a.md", "b.md"}, wantErr: "expected one path, got 2"},
		{name: "config takes no args", args: []string{"config", "x"}, wantErr: "config takes no arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse(%q) error = %v, want containing %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.args, err)
			}
			if got != tt.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}
