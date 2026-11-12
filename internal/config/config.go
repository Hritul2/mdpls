// Package config loads mdpls settings from ~/.mdpls/config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2/styles"
)

// FileName is the config file inside the config directory.
const FileName = "config.json"

// Theme controls how rendered pages look. Every value is plain CSS.
type Theme struct {
	// ColorScheme is "dark" or "light": it tells the browser how to draw
	// scrollbars and form controls to match the colours below.
	ColorScheme string `json:"color_scheme"`
	Background  string `json:"background"`
	Surface     string `json:"surface"`
	Text        string `json:"text"`
	Muted       string `json:"muted"`
	Border      string `json:"border"`
	Accent      string `json:"accent"`
	FontBody    string `json:"font_body"`
	FontMono    string `json:"font_mono"`
	FontSize    string `json:"font_size"`
	LineHeight  string `json:"line_height"`
	MaxWidth    string `json:"max_width"`
	Radius      string `json:"radius"`
	// CodeStyle is a chroma style name, e.g. "github-dark", "dracula", "nord".
	CodeStyle string `json:"code_style"`
}

// Config is the whole settings file.
type Config struct {
	// Port is the first port tried. The default, 1112, is the date mdpls was
	// born: Nov 12 (2026).
	Port        int      `json:"port"`
	OpenBrowser bool     `json:"open_browser"`
	AllowHTML   bool     `json:"allow_html"`
	Ignore      []string `json:"ignore"`
	Theme       Theme    `json:"theme"`
}

// Default returns the built-in settings.
func Default() Config {
	return Config{
		Port:        1112,
		OpenBrowser: true,
		AllowHTML:   true,
		Ignore:      []string{"node_modules", "vendor", "dist", "build", "target", "coverage", "__pycache__"},
		Theme: Theme{
			ColorScheme: "dark",
			Background:  "#0f0f0f",
			Surface:     "#1a1a1a",
			Text:        "#f0f0f0",
			Muted:       "#949494",
			Border:      "#2e2e2e",
			Accent:      "#fb7185",
			FontBody:    `-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif`,
			FontMono:    `ui-monospace, "SF Mono", SFMono-Regular, Menlo, Consolas, monospace`,
			FontSize:    "16px",
			LineHeight:  "1.7",
			MaxWidth:    "768px",
			Radius:      "12px",
			CodeStyle:   "github-dark",
		},
	}
}

// Dir returns the config directory: $MDPLS_HOME, or ~/.mdpls.
func Dir() (string, error) {
	if d := os.Getenv("MDPLS_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".mdpls"), nil
}

// Load reads dir/config.json. If it doesn't exist, it is created with the
// defaults and created is true. Fields missing from the file keep their defaults.
func Load(dir string) (cfg Config, created bool, err error) {
	path := filepath.Join(dir, FileName)
	cfg = Default()

	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := Save(dir, cfg); err != nil {
			return cfg, false, err
		}
		return cfg, true, nil
	}
	if err != nil {
		return cfg, false, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, false, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, false, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, false, nil
}

// Save writes cfg to dir/config.json, creating dir if needed.
func Save(dir string, cfg Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Validate checks values that would break the server or the page.
func (c Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	t := c.Theme
	fields := map[string]string{
		"background": t.Background, "surface": t.Surface, "text": t.Text,
		"muted": t.Muted, "border": t.Border, "accent": t.Accent,
		"font_body": t.FontBody, "font_mono": t.FontMono, "font_size": t.FontSize,
		"line_height": t.LineHeight, "max_width": t.MaxWidth, "radius": t.Radius,
	}
	for name, v := range fields {
		// Values are dropped into a <style> block: refuse anything that could
		// close a declaration, a rule, or the style element.
		if strings.ContainsAny(v, ";{}<>") {
			return fmt.Errorf("theme.%s contains a character that isn't allowed (; { } < >): %q", name, v)
		}
	}
	if t.ColorScheme != "dark" && t.ColorScheme != "light" {
		return fmt.Errorf("theme.color_scheme must be \"dark\" or \"light\", got %q", t.ColorScheme)
	}
	if _, ok := styles.Registry[t.CodeStyle]; !ok {
		return fmt.Errorf("theme.code_style %q is not a known chroma style (try github-dark, dracula, monokai, nord)", t.CodeStyle)
	}
	return nil
}
