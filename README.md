# mdpls

**Markdown, please.** Preview every markdown file in a repo in your browser —
rendered GitHub-style, linked together, and refreshing live as you edit.

```sh
cd your-repo
mdpls README.md
```

```
mdpls  serving /Users/you/your-repo
       http://127.0.0.1:1112/README.md
       config /Users/you/.mdpls/config.json
       Ctrl-C to stop
```

- **Whole repo, not one file.** mdpls serves the git root, so relative links
  between docs (`[setup](docs/setup.md#install)`), images, and anchors just work.
  `/` shows every markdown file as a collapsible folder tree (folders first,
  file counts, expand/collapse all). Open folders are remembered across
  refreshes.
- **Live.** Save a file and the open page updates in place — no reload, no lost
  scroll position. New files show up in the index on their own.
- **GitHub-flavored.** Tables, task lists, strikethrough, autolinks, footnotes,
  heading anchors, front matter hidden, syntax highlighting for ~250 languages.
- **One small binary.** No runtime, no Node, no pandoc. Ctrl-C and it's gone.
- **Yours to style.** Colours, fonts, width and code theme live in a JSON file.
- **Local only.** Binds to `127.0.0.1`, serves nothing outside the root, makes no
  network requests of its own.

## Install

**Homebrew** (macOS, Linux)

```sh
brew install Hritul2/tap/mdpls
```

**Go** (1.25+)

```sh
go install github.com/Hritul2/mdpls@latest
```

**Prebuilt binaries and Linux packages** — download from the
[releases page](https://github.com/Hritul2/mdpls/releases): `.tar.gz` for
macOS/Linux, `.zip` for Windows, plus `.deb`, `.rpm` and `.apk`.

```sh
sudo dpkg -i mdpls_*_linux_amd64.deb     # Debian/Ubuntu
sudo rpm -i mdpls_*_linux_amd64.rpm      # Fedora/RHEL
```

## Usage

```
mdpls [path] [options]
mdpls config
```

| Command | What it does |
|---|---|
| `mdpls` | serve the current repo, open the index |
| `mdpls docs/guide.md` | serve the repo that file is in, open that file |
| `mdpls docs/` | serve the repo, open the `docs/` listing |
| `mdpls config` | print where the config lives and what's in it |

| Option | |
|---|---|
| `--port <n>` | first port to try (default from config) |
| `--no-open` | don't open the browser |
| `-v`, `--version` | print the version |
| `-h`, `--help` | show help |

The served root is the **git root** of the path you give (or of the current
directory). Outside a git repo, it's the directory itself.

Press **Ctrl-C** to stop. Get a command wrong and mdpls tells you what's off and
points you at `mdpls --help`.

### The port

mdpls always starts on the same port — **1112**, after the day it was born
(Nov 12, 2026) — so the URL stays bookmarkable. It only moves to the next free
port (1113, 1114, …) when 1112 is already taken, and says so. Change the
default in the config, or per run with `--port`.

## Configuration

The first run creates `~/.mdpls/config.json` with every setting filled in.
Edit it and restart mdpls. Set `MDPLS_HOME` to keep it somewhere else.
Settings you delete fall back to the defaults.

```json
{
  "port": 1112,
  "open_browser": true,
  "allow_html": true,
  "ignore": ["node_modules", "vendor", "dist", "build", "target", "coverage", "__pycache__"],
  "theme": {
    "color_scheme": "dark",
    "background": "#0f0f0f",
    "surface": "#1a1a1a",
    "text": "#f0f0f0",
    "muted": "#949494",
    "border": "#2e2e2e",
    "accent": "#fb7185",
    "font_body": "-apple-system, BlinkMacSystemFont, \"Segoe UI\", system-ui, sans-serif",
    "font_mono": "ui-monospace, \"SF Mono\", SFMono-Regular, Menlo, Consolas, monospace",
    "font_size": "16px",
    "line_height": "1.7",
    "max_width": "768px",
    "radius": "12px",
    "code_style": "github-dark"
  }
}
```

| Setting | Meaning |
|---|---|
| `port` | first port to try |
| `open_browser` | open the browser on start (`--no-open` overrides for one run) |
| `allow_html` | keep raw HTML in markdown (`<details>`, `<img>`, `<br>`…). Set `false` for untrusted repos |
| `ignore` | directory names never listed or watched (hidden `.dirs` are always skipped) |
| `theme.color_scheme` | `dark` or `light` — matches scrollbars and form controls to your colours |
| `theme.background` … `theme.accent` | any CSS colour. `accent` colours links and **bold** text |
| `theme.font_*`, `font_size`, `line_height`, `max_width`, `radius` | any CSS value |
| `theme.code_style` | a [chroma style](https://xyproto.github.io/splash/docs/): `github-dark`, `dracula`, `monokai`, `nord`, `github`, … |

A light theme, for example:

```json
"theme": {
  "color_scheme": "light",
  "background": "#ffffff", "surface": "#f6f8fa", "text": "#1f2328",
  "muted": "#59636e", "border": "#d1d9e0", "accent": "#0969da",
  "code_style": "github"
}
```

## How it works

- Markdown is rendered with [goldmark](https://github.com/yuin/goldmark) (GFM)
  and highlighted with [chroma](https://github.com/alecthomas/chroma).
- Pages subscribe to `/__mdpls/events` (server-sent events). When a file changes,
  the page fetches its new content and swaps it in.
- Changes are found by polling, not OS file events: known markdown files are
  checked every 500 ms and the tree is re-scanned every 3 s for new ones. This
  avoids running out of file descriptors on large repos (macOS kqueue needs one
  per watched file).

### Security

- Listens on `127.0.0.1` only, and answers only requests addressed to
  `127.0.0.1`, `localhost` or `::1` (blocks DNS-rebinding attacks).
- Never serves anything outside the root, including through `..` or symlinks.
- Read-only: GET and HEAD only.
- With `allow_html: true`, HTML inside markdown runs in the page like it would in
  any local HTML file. Turn it off when previewing repos you don't trust.

## Development

```sh
go test -race ./...     # tests
go run . README.md      # try it
```

Layout:

| Package | |
|---|---|
| `internal/cli` | argument parsing and help text |
| `internal/config` | `~/.mdpls/config.json`: defaults, merging, validation |
| `internal/port` | fixed port that moves only when taken |
| `internal/root` | finds the git root |
| `internal/render` | markdown → HTML, page template, theme CSS |
| `internal/server` | HTTP handler, index pages, live reload |
| `internal/app` | the command: startup, banner, Ctrl-C shutdown |

### Releasing

Releases are built by [GoReleaser](https://goreleaser.com) from a version tag:

```sh
git tag v0.1.0 && git push origin v0.1.0
```

The `release` workflow then builds binaries for macOS, Linux and Windows
(amd64 + arm64), attaches archives, `.deb`/`.rpm`/`.apk` packages and checksums
to the GitHub release, and updates the Homebrew formula.

One-time setup for Homebrew:

1. Create a public repo `Hritul2/homebrew-tap` (empty is fine).
2. Create a fine-grained token with **Contents: read and write** on that repo.
3. Add it to this repo as the secret `HOMEBREW_TAP_GITHUB_TOKEN`.

## License

[MIT](LICENSE)
