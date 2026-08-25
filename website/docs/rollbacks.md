# Rollbacks

Before overwriting a binary during `install` or `sync`, Pkgline copies the existing executable to `.bak` in the same directory.

```bash
pkgline rollback my-tool
```

This restores `my-tool` from `my-tool.bak`.
