# CLI Reference

## `install`

```bash
pkgline install <uri>
```

URI formats: `gh:user/repo`, `gl:group/repo`, `cb:user/repo`, `sh:user/repo`, `user/repo` (GitHub), full `https://` or `git@` URLs, or a local path.

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
