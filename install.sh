#!/usr/bin/env sh
set -e

REPO="iamvxrn/pkgline"
BINARY="pkgline"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    linux)
        ;;
    darwin)
        ;;
    *)
        echo "Unsupported operating system: $OS"
        exit 1
        ;;
esac

TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$TAG" ]; then
    TAG="v0.1.0"
fi

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${FILENAME}"
INSTALL_DIR="${HOME}/.local/bin"

echo "Downloading ${BINARY} ${TAG}..."
TMP_DIR=$(mktemp -d)
curl -sSL "$URL" -o "${TMP_DIR}/${BINARY}.tar.gz"

tar -xzf "${TMP_DIR}/${BINARY}.tar.gz" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"
rm -rf "$TMP_DIR"

echo "${BINARY} successfully installed to ${INSTALL_DIR}/${BINARY}"
echo "Make sure ${INSTALL_DIR} is in your PATH."
