# Pkgline in your terminal

Pkgline installs developer tools from Git repositories into your user directory — no `sudo`, no central registry.

<div style="display: flex; gap: 12px; margin-top: 1.5rem; margin-bottom: 1.5rem; flex-wrap: wrap; align-items: center;">
  <a href="/quickstart" style="background-color: var(--vp-button-brand-bg); color: var(--vp-button-brand-text); padding: 8px 16px; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 14px;">Quickstart</a>
  <a href="https://github.com/iamvxrn/pkgline" style="background-color: var(--vp-button-alt-bg); color: var(--vp-button-alt-text); padding: 8px 16px; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 14px; border: 1px solid var(--vp-button-alt-border);">GitHub</a>
  <div style="background-color: #161618; border: 1px solid #3c3f44; border-radius: 8px; padding: 6px 12px; font-family: monospace; font-size: 13px;">
    <span style="color: #8b949e;">$</span> curl -fsSL https://pkgline.pages.dev/install.sh | sh
  </div>
</div>

[Other install options →](/install)

---

## Try it

```bash
pkgline install gh:username/repository
pkgline list
pkgline sync
pkgline rollback my-tool
```
