# Changelog

## [Unreleased]

Honor `bin_dir` / `apps_dir` from config.toml. `sync` backs up the binary, skips git pull for local installs, and does not rebuild when `rev-parse` fails.

CI, lint, and tests: gofmt, golangci-lint, govulncheck, and unit coverage for config, db, git, path, ui, and the CLI helpers.

## [0.3.0] - 2026-08-26

Forge prefixes, machine-readable output, and Make/CMake builds.

- `gl:`, `cb:`, and `sh:` URI prefixes (GitLab, Codeberg, Sourcehut)
- `--json` on `list`, `doctor`, and `version`
- Native `make` / `cmake` languages; inferred from Makefile / CMakeLists.txt when `language` is omitted
- Tests isolate state with `PKGLINE_ROOT` (and honor `PKGLINE_BIN` / `PKGLINE_APPS`)

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
