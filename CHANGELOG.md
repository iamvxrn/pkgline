# Changelog

## [0.2.0] - 2026-08-25

Expanded build support, lifecycle commands, and documentation site.

- `remove`, `rollback`, and `sync` commands
- Rust, cbld/c/cpp native builds and install-script fallback
- `doctor` and shell `completion` (bash, zsh, fish)
- Config aliases (`cbld`, `pkgline`, `muth`, `runa`)
- `install.ps1`, `uninstall.sh`, tests, CI, and VitePress docs

## [0.1.0] - 2026-08-19

Initial release with Git-based installs and Go builds.

- `install` from Git URLs, `gh:user/repo`, or local directories
- `go build` for packages with `language = "go"`
- `list` to show installed packages
