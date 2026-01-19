#!/usr/bin/env bash
set -euo pipefail

OWNER="shinshin86"
REPO="vpeakserver"
BIN_NAME="vpeakserver"

usage() {
  cat <<EOF
Install ${BIN_NAME}
usage:
  curl -fsSL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/install.sh | bash
optional:
  curl -fsSL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/install.sh | bash -- vX.Y.Z
EOF
}

err() {
  echo "error: $*" >&2
  exit 1
}

info() {
  echo "$*"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "$1 is required but not found in PATH."
}

if [ $# -gt 1 ]; then
  usage
  err "too many arguments"
fi

TAG="latest"
if [ $# -eq 1 ]; then
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    v*.*.*)
      TAG="$1"
      ;;
    *)
      usage
      err "invalid version format: $1 (expected vX.Y.Z or 'latest')"
      ;;
  esac
fi

need_cmd uname
need_cmd curl
need_cmd tar
need_cmd awk
need_cmd install
need_cmd mktemp
need_cmd date

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin) ;;
  *)
    err "unsupported OS: ${OS}"
    ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  arm64|aarch64)
    ARCH="arm64"
    ;;
  x86_64|amd64)
    ARCH="amd64"
    ;;
  *)
    err "unsupported architecture: ${ARCH_RAW}"
    ;;
esac

if [ "$TAG" = "latest" ]; then
  TAG="$(curl -fsSL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" | awk -F '\"' '/tag_name/ {print $4}')"
  if [ -z "$TAG" ]; then
    err "failed to resolve latest version"
  fi
fi

ASSET_NAME="${BIN_NAME}_${TAG}_${OS}_${ARCH}"
ASSET="${ASSET_NAME}.tar.gz"
CHECKSUMS="checksums.txt"
BASE_URL="https://github.com/${OWNER}/${REPO}/releases/download/${TAG}"

TMP_DIR="$(mktemp -d -t ${BIN_NAME}-install.XXXXXXXX)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

info "Downloading checksums..."
curl -fsSL "${BASE_URL}/${CHECKSUMS}" -o "${TMP_DIR}/${CHECKSUMS}"

EXPECTED_SHA="$(awk "/${ASSET}\$/ {print \$1}" "${TMP_DIR}/${CHECKSUMS}")"
if [ -z "${EXPECTED_SHA}" ]; then
  err "checksum entry for ${ASSET} not found"
fi

info "Downloading ${ASSET}..."
curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP_DIR}/${ASSET}"

if command -v shasum >/dev/null 2>&1; then
  ACTUAL_SHA="$(shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SHA="$(sha256sum "${TMP_DIR}/${ASSET}" | awk '{print $1}')"
else
  err "shasum or sha256sum is required but not found in PATH."
fi

if [ "${EXPECTED_SHA}" != "${ACTUAL_SHA}" ]; then
  err "checksum mismatch: expected ${EXPECTED_SHA}, got ${ACTUAL_SHA}"
fi

info "Extracting..."
tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}"

BIN_PATH="${TMP_DIR}/${ASSET_NAME}"
if [ ! -f "${BIN_PATH}" ]; then
  err "binary not found after extraction: ${BIN_PATH}"
fi

BIN_DIR="${HOME}/.local/bin"
mkdir -p "${BIN_DIR}"

TARGET_PATH="${BIN_DIR}/${BIN_NAME}"
if [ -f "${TARGET_PATH}" ]; then
  BACKUP="${TARGET_PATH}.bak-$(date +%Y%m%d%H%M%S)"
  cp "${TARGET_PATH}" "${BACKUP}"
  info "Backed up existing binary to ${BACKUP}"
fi

install -m 0755 "${BIN_PATH}" "${TARGET_PATH}"
info "Installed to ${TARGET_PATH}"

if [[ ":${PATH}:" != *":${BIN_DIR}:"* ]]; then
  info "Add ${BIN_DIR} to your PATH to run ${BIN_NAME} from the terminal."
  info "Example (bash/zsh): echo 'export PATH=\"${BIN_DIR}:\$PATH\"' >> ~/.bashrc"
fi

info "Done. Run: ${BIN_NAME} --version"
