# Changelog

## [Unreleased]

## [0.4.0] - 2026-08-30

The `Pkglinefile` + `binary cache` + `search` release — pkgline as `brew for sources`.

- **Pkglinefile** — declarative toolchain: `Pkglinefile` / `Pkglinefile.toml` (`--lang`/`--exec` per line, `//` comments, `ResolvedURI` for `./` paths), `pkgline bootstrap [--file <path>] [--dry-run]` walks up and installs sequentially (`internal/pkglinefile`)
- **Binary cache** — global `~/.pkgline/cache/prebuilt/<hash>/<bin>` with key `sha256(uri|commit|version|lang|exec|GOOS|GOARCH|goVersion)[:16]`; hit on `install` and `sync` (single + 4-worker parallel), best-effort store/restore (`internal/cache`)
- **Search** — `pkgline search <query> [--limit N] [--json]` via GitHub code search `pkgline.toml in:path <query>` with `GITHUB_TOKEN`/`GH_TOKEN`, deduped repos, stars/description (`internal/search`)
- **Zig / Node** — native builders: `build.zig` → `zig build` → `zig-out/bin/<exec>`, `package.json` → `npm ci`/`install` + `npm run build` → `bin` field or `dist/` fallback; inferred from `build.zig`/`package.json`, validated as `zig`/`node` (`builder.go`, `manifest.go`)
- **Run without install** — `pkgline run` / `r <uri> [-- --]` clones to temp, `BuildAndInstall` to temp bin, exec with forwarded args, no inventory
- **Publish** — `pkgline publish [--force] [--yes]` interactive `pkgline.toml` generator inferring from `go.mod`/`Cargo.toml`/`cbld.toml`/`CMake`/`Makefile`
- **Parallel sync** — 4-worker goroutine pool for `sync` of N packages
- **Core fixes since 0.3.0** — honor `bin_dir`/`apps_dir` from `config.toml`, `sync` backs up binary and skips git pull for local installs, `install`/`remove`/`rollback --help` handling, language inference, `--lang`/`--exec` overrides, submodules, `@tag` pinning, `main_path`, Windows `install.ps1`, inventory lock, rollback metadata, SIGINT cleanup
- CI, lint, tests: `gofmt`, `golangci-lint` (`errcheck`, `staticcheck`), `govulncheck`, coverage for `cache`, `pkglinefile`, `search`, `config`, `db`, `git`, `path`, `ui`

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
