# Quickstart

## 1. Install a package

```bash
pkgline install gh:username/repo
```

Pkgline clones the repo to `~/.pkgline/apps/<name>`, reads `pkgline.toml`, builds the binary, and links it in `~/.pkgline/bin`.

## 2. Check what's installed

```bash
pkgline list
```

## 3. Update packages

```bash
pkgline sync
```

Update one package:

```bash
pkgline sync repo
```
