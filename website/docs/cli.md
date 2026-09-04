# CLI Reference

## `install`

```bash
pkgline install <uri>
pkgline install --lang go --exec mytool <uri>
```

URI formats: `gh:user/repo`, `gl:group/repo`, `cb:user/repo`, `sh:user/repo`, `user/repo` (GitHub), full `https://` or `git@` URLs, or a local path.

`--lang` / `--language` overrides inferred or manifest language (`go`, `rust`, `zig`, `node` / `nodejs` / `js` / `ts`, `cbld`, `c`, `cpp`, `make`, `cmake`). `--exec` / `--executable` overrides the binary name. Both accept `--lang=go` form. Flags may appear before or after the URI. Use `--` before a URI that begins with `-`.

Pin a ref by appending `@<branch|tag|commit>`, e.g. `pkgline install gh:user/repo@v1.2.0`.

## `run`

```bash
pkgline run <uri>
pkgline run gh:user/repo -- --flag value
```

Aliased to `r`. Clones the package to a temporary directory, builds it into a
temporary bin directory, executes it with any arguments after `--`, then
discards both. Nothing is written to `~/.pkgline` and nothing is added to the
inventory. Accepts the same `--lang` / `--exec` flags as `install`.

## `publish`

```bash
pkgline publish
pkgline publish --force --yes
```

Interactively generates a `pkgline.toml` for the current directory, inferring
the language from `go.mod` / `Cargo.toml` / `cbld.toml` / `CMakeLists.txt` /
`Makefile`. `--force` / `-f` overwrites an existing `pkgline.toml`; `--yes` /
`-y` accepts the inferred values without prompting.

## `bootstrap`

```bash
pkgline bootstrap
pkgline bootstrap --file ./Pkglinefile --dry-run
pkgline bootstrap --yes
```

Prompts `Install N package(s) from Pkglinefile? [y/N]` before doing anything.
`--yes` / `-y` skips the prompt; `--dry-run` / `-n` lists what would be
installed and never prompts. `--file` / `-f` points at a specific file.

Installs all packages listed in `Pkglinefile` (walks up from `cwd`). `Pkglinefile` is a plain text file, one package spec per line (same URI forms as `install`, plus `--lang`/`--exec` per line). Lines starting with `#` and blank lines are ignored (`#` only — `//` is not a comment marker). A trailing ` # ...` outside quotes is stripped. Also supports `Pkglinefile.toml` with `packages = ["gh:a/b", "..."]` or `[[packages]]` tables.

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

Aliased to `rm` and `uninstall`. Runs the manifest's `[scripts] uninstall` hook
only when the package was installed by its `[scripts] install` hook (see
[the manifest reference](/manifest#language-wins-over-install)), then deletes
the binary, its `.bak`, the app directory, and the inventory record.

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

## `search`

```bash
pkgline search <query>
pkgline search --limit 5 json parser
pkgline search --json "http client" | jq
```

Searches GitHub for repositories containing `pkgline.toml` (via `pkgline.toml in:path <query>` code search). Deduplicates by repo, shows stars and `pkgline install` hint. Requires `GITHUB_TOKEN`/`GH_TOKEN` for code search auth; without it GitHub returns 401/rate-limit.

## `cache`

Binary cache lives at `~/.pkgline/cache/prebuilt/<hash>/<bin>` where `<hash>` is `sha256(uri|commit|version|lang|exec|GOOS|GOARCH|goVersion|toolchainVersion)[:16]` — `toolchainVersion` is `rustc --version` for `rust` and `$CC`/`$CXX --version` for `c`/`cpp`/`cbld`, so a compiler upgrade invalidates the cache. Populated after each successful build, consulted on `install` and `sync`. Best-effort — cache failures never fail an install.

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
