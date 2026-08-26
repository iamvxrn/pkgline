# Overview

Pkgline is a user-space package manager for Linux and macOS. It installs developer tools directly from git repositories without needing `sudo`.

## Why Pkgline?

- **No sudo:** Binaries go into `~/.pkgline/bin` (add it to your `PATH`).
- **Git-first:** Clone from forge shorthand (`gh:`, `gl:`, `cb:`, `sh:`), full Git URLs, or local directories.
- **Native builds:** Go, Rust, cbld/C/C++, Make, and CMake. Make/CMake can be inferred from `Makefile` / `CMakeLists.txt` when `language` is omitted.
- **Script fallback:** Repos can ship `install.sh` / `uninstall.sh` hooks in `pkgline.toml`.

## Architecture

```mermaid
graph TD
    A[Git remotes] -->|clone| B(Pkgline)
    B -->|read pkgline.toml| C{Build type}
    C -->|go / rust / cbld / make / cmake| D[Compile]
    C -->|install.sh| E[Script hook]
    D --> F[~/.pkgline/bin]
    E --> F
```
