# Installation

## curl (Linux / macOS)

```bash
curl -fsSL https://pkgline.pages.dev/install.sh | sh
```

The script downloads the latest release binary from GitHub into `~/.local/bin`.

## PowerShell (Windows)

```powershell
Invoke-Expression (Invoke-WebRequest -Uri "https://pkgline.pages.dev/install.ps1" -UseBasicParsing).Content
```

## From source

```bash
git clone https://github.com/iamvxrn/pkgline.git
cd pkgline
go build -o pkgline .
```

## PATH

Pkgline installs packages to `~/.pkgline/bin`. Add it to your shell config:

```bash
export PATH="$HOME/.pkgline/bin:$PATH"
```
