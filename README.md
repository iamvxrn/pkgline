<p align="center">
  <img src="website/docs/public/social.png" alt="pkgline — Install developer tools from Git" width="720">
</p>

# Pkgline

A tool for downloading and building executables from Git repositories into the user's local directory.

[![Documentation](https://img.shields.io/badge/docs-pkgline.pages.dev-10b981.svg)](https://pkgline.pages.dev)
[![OS Matrix](https://img.shields.io/badge/OS-Linux%20%7C%20macOS%20%7C%20Windows-10b981.svg)](#)

## Features

- Installs binaries to `~/.pkgline/bin` (add it to your `PATH`).
- Resolves GitHub / GitLab / Codeberg / Sourcehut shorthand (`gh:`, `gl:`, `cb:`, `sh:`) and full Git URLs.
- Native builds for Go, Rust, cbld/C/C++, Make, and CMake; custom `install.sh` scripts as fallback.
- Backup binaries (`.bak`) with `rollback` after updates.

## Quick Start

### Installation

```bash
curl -fsSL https://pkgline.pages.dev/install.sh | sh
```

Windows:

```powershell
Invoke-Expression (Invoke-WebRequest -Uri "https://pkgline.pages.dev/install.ps1" -UseBasicParsing).Content
```

### Usage

```bash
pkgline install gh:user/repository
pkgline list
pkgline remove package_name
pkgline sync
pkgline update package_name
pkgline rollback package_name
pkgline doctor
pkgline completion bash
```

## Documentation

[https://pkgline.pages.dev](https://pkgline.pages.dev)

## License

MIT
