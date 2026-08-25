# URI Formats

Pkgline resolves package sources from Git URLs, GitHub shorthand, or local paths.

| Input | Resolved to |
|---|---|
| `gh:user/repo` | `https://github.com/user/repo.git` |
| `user/repo` | GitHub when not a local path |
| `https://...` / `git@...` | used as-is |
| `./path` / `/absolute/path` | local directory copy |

Other forges (GitLab, Codeberg, Sourcehut) work via full `https://` or `git@` clone URLs. Short prefixes like `gl:` are not implemented yet.
