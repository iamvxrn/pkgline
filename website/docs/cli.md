# CLI Reference

## `install`

```bash
pkgline install <uri>
pkgline install --lang go --exec mytool <uri>
```

URI formats: `gh:user/repo`, `gl:group/repo`, `cb:user/repo`, `sh:user/repo`, `user/repo` (GitHub), full `https://` or `git@` URLs, or a local path.

`--lang` / `--language` overrides inferred or manifest language (`go`, `rust`, `cbld`, `c`, `cpp`, `make`, `cmake`). `--exec` / `--executable` overrides the binary name. Both accept `--lang=go` form. Flags may appear before or after the URI.

## `bootstrap`

```bash
pkgline bootstrap
pkgline bootstrap --file ./Pkglinefile --dry-run
```

Installs all packages listed in `Pkglinefile` (walks up from `cwd`). `Pkglinefile` is a plain text file, one package spec per line (same URI forms as `install`, plus `--lang`/`--exec` per line). Lines starting with `#` and blank lines are ignored. Also supports `Pkglinefile.toml` with `packages = ["gh:a/b", "..."]` or `[[packages]]` tables.

Example `Pkglinefile`:
```
gh:iamvxrn/cbld
gh:golangci/golangci-lint@v1.64.8 --lang go
./tools/my-local-tool --lang rust
```

## `remove`

```bash
pkgline remove <package-name>
```

## `rollback`

```bash
pkgline rollback <package-name>
```

Restores the previous binary from `.bak` if one exists.

## `sync`

```bash
pkgline sync [package-name]
```

Rebuilds only when `git pull` moved, version changed, or commit differs. Identical `OS+arch+commit+version+lang` builds hit the global cache at `~/.pkgline/cache/prebuilt` and skip recompilation.

## `cache`

Binary cache lives at `~/.pkgline/cache/prebuilt/<hash>/<bin>` where `<hash>` is `sha256(uri|commit|version|lang|exec|GOOS|GOARCH|goVersion)[:16]`. Populated after each successful build, consulted on `install` and `sync`. Best-effort — cache failures never fail an install.

## `list`

```bash
pkgline list
pkgline list --json
```

## `doctor`

```bash
pkgline doctor
pkgline doctor --json
```

## `version`

```bash
pkgline version
pkgline version --json
```

## `completion`

```bash
pkgline completion bash
pkgline completion zsh
pkgline completion fish
```
