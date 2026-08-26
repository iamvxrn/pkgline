# URI Formats

Pkgline resolves package sources from Git URLs, forge shorthand, aliases, or local paths.

| Input | Resolved to |
|---|---|
| `gh:user/repo` | `https://github.com/user/repo.git` |
| `gl:group/repo` | `https://gitlab.com/group/repo.git` |
| `cb:user/repo` | `https://codeberg.org/user/repo.git` |
| `sh:user/repo` | `https://git.sr.ht/~user/repo.git` |
| `user/repo` | GitHub when not a local path |
| `https://...` / `git@...` | used as-is |
| `./path` / `/absolute/path` | local directory copy |

`sh:~user/repo` is accepted too (the `~` is not doubled).
