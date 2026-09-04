# The pkgline.toml Manifest

Drop a `pkgline.toml` in the repo root when Pkgline needs explicit build metadata.

## Example

A natively built package needs no `[scripts]` at all:

```toml
[package]
name = "my-tool"
version = "1.0.0"
language = "go"
executable = "my-tool"
```

A package with no supported language supplies its own installer instead:

```toml
[package]
name = "my-tool"
version = "1.0.0"

[scripts]
install = "install.sh"
uninstall = "uninstall.sh"
```

## `[package]`

- `name` — package name (required)
- `version` — package version string
- `language` — `go`, `rust`, `zig`, `node` (`js`/`ts`), `cbld`, `c`, `cpp`, `make`, or `cmake`. If omitted and there is no install script, Pkgline infers from `go.mod` / `Cargo.toml` / `build.zig` / `package.json` / `cbld.toml` / `CMakeLists.txt` / `Makefile`. Override at install time with `--lang`.
- `executable` — binary name (defaults to `name`). Override at install time with `--exec`.
- `main_path` — Go only. The package to build, passed to `go build` (default `.`, falling back to `./cmd/<name>` when that directory exists and there is no top-level `main.go`).

`name` and `executable` are used as single path components under `~/.pkgline/apps` and `~/.pkgline/bin`. They must be plain file names: a value containing `/`, `\`, or `..`, or an absolute path, is rejected.

## `[scripts]`

`[scripts]` is the **fallback** used when `language` is absent or is not one of
the natively supported languages.

- `install` — script path (e.g. `install.sh`), relative to the package root
- `uninstall` — script path run on `pkgline remove`

### `language` wins over `install`

If `language` names a supported native builder, Pkgline builds natively and
**never runs `install`**. Setting both is a mistake: the script is dead
configuration, and Pkgline warns about it at install time. Pick one.

For symmetry, `uninstall` runs only for packages that were installed *by* their
`install` script. An `uninstall` hook exists to undo what its `install` hook
did, so Pkgline does not run it against a natively built package — it would
undo something that never happened. This matters in practice because these
scripts are usually a project's standalone `curl | sh` installer and its
counterpart, which delete a binary from a hardcoded location such as
`~/.local/bin` rather than from `$PKGLINE_BIN`.

### Script environment

An install or uninstall script that Pkgline runs receives:

| Variable | Meaning |
| --- | --- |
| `PKGLINE_BIN` | Directory the binary must be installed into |
| `PKGLINE_APP_ROOT` | The package source checkout |
| `PKGLINE_PACKAGE_NAME` | `[package] name` |
| `PKGLINE_PACKAGE_VERSION` | `[package] version` |
| `PKGLINE_EXECUTABLE` | Resolved executable name |

Install to `"$PKGLINE_BIN/$PKGLINE_EXECUTABLE"`. A script that hardcodes another
directory installs somewhere Pkgline will not find, list, update, or remove.
