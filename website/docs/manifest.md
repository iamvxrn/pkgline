# The pkgline.toml Manifest

Drop a `pkgline.toml` in the repo root when Pkgline needs explicit build metadata.

## Example

```toml
[package]
name = "my-tool"
version = "1.0.0"
language = "go"
executable = "my-tool"

[scripts]
install = "install.sh"
uninstall = "uninstall.sh"
```

## `[package]`

- `name` — package name (required)
- `version` — package version string
- `language` — `go`, `rust`, `cbld`, `c`, `cpp`, `make`, or `cmake`. If omitted and there is no install script, Pkgline infers from `go.mod` / `Cargo.toml` / `cbld.toml` / `CMakeLists.txt` / `Makefile`. Override at install time with `--lang`.
- `executable` — binary name (defaults to `name`). Override at install time with `--exec`.

## `[scripts]`

Optional hooks when native builds are not used or for cleanup:

- `install` — script path (e.g. `install.sh`)
- `uninstall` — script path run on `pkgline remove`
