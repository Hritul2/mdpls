# mdpls — build plan

Goal: a small, dependency-free CLI that previews a repo's markdown in the
browser, live, installable with Homebrew and published as open source.

## Requirements

- Serve every markdown file under a repo root (git root of the given path);
  links between files, images and anchors work.
- Live refresh in the browser when files change.
- Fixed default port **1112** (Nov 12, 2026 — the day it was started); move to
  the next free port only when it's taken.
- Settings in `~/.mdpls/config.json`: port, browser, raw HTML, ignored
  directories, and the full theme (colours, fonts, layout, code style).
- Ctrl-C stops it; wrong usage explains itself and points at `--help`.
- Publishable: Homebrew tap, `go install`, release binaries, deb/rpm/apk.

## Design decisions

| Decision | Why |
|---|---|
| goldmark + chroma, no pandoc | one self-contained binary; `brew install` needs nothing else |
| Poll for changes instead of fsnotify | kqueue (macOS) needs a file descriptor per watched file; big repos run out |
| SSE + fetch-and-swap | live refresh without a websocket dependency or page reloads |
| Probe ports by connecting, then bind | binding alone can succeed beside a wildcard listener and shadow it |
| Loopback Host check | blocks DNS rebinding |
| Request contexts derive from the Ctrl-C context | open live-reload streams end immediately on shutdown |
| System fonts by default | works offline, no third-party requests |

## Build order (test-first for each)

1. `cli` — parsing, usage errors
2. `config` — defaults, merge, validation, `$MDPLS_HOME`
3. `port`, `root`
4. `render` — GFM, front matter, highlighting, CSS, page template
5. `server` — routing, index, static files, traversal/symlink/host safety, live events
6. `app` + `main` — banner, browser, shutdown, `config` command
7. GoReleaser, CI and release workflows, README

## Publishing

- Public repo `github.com/Hritul2/mdpls`.
- Tag `vX.Y.Z` → release workflow → GitHub release + Homebrew formula in
  `Hritul2/homebrew-tap` (needs the `HOMEBREW_TAP_GITHUB_TOKEN` secret).
- Possible later channels: Scoop and winget (Windows), AUR (Arch), nixpkgs,
  homebrew-core once the project is established.
