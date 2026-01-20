#!/usr/bin/env bash
set -e

OWNER="shinshin86"
REPO="vpeakserver"
BIN_NAME="vpeakserver"
BIN_DIR="${BIN_DIR:-${HOME}/.local/bin}"

usage() {
  cat <<EOF
Usage:
  curl -fsSL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/install.sh | bash
  curl -fsSL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/install.sh | bash -- vX.Y.Z
  BIN_DIR=/custom/path curl -fsSL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/install.sh | bash
EOF
}

err() {
  echo "error: $*" >&2
  exit 1
}

for cmd in curl tar install shasum; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    err "${cmd} is required but not installed."
  fi
done

version="${1:-latest}"
if [ "${version}" != "latest" ] && ! [[ "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  usage
  err "Invalid version format. Use vX.Y.Z or 'latest'"
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "${os}" != "darwin" ]; then
  err "Unsupported OS: ${os}"
fi

arch=$(uname -m)
case "${arch}" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64) arch="amd64" ;;
  *) err "Unsupported architecture: ${arch}" ;;
esac

if [ "${version}" = "latest" ]; then
  version=$(curl --retry 3 -fL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
fi

if [ -z "${version}" ]; then
  err "Failed to determine the latest version."
fi

asset="${BIN_NAME}_${version}_${os}_${arch}.tar.gz"
base_url="https://github.com/${OWNER}/${REPO}/releases/download/${version}"

tmp_dir=$(mktemp -d)
trap 'rm -rf "${tmp_dir}"' EXIT

echo "Downloading checksums..."
curl --retry 3 -fL "${base_url}/checksums.txt" -o "${tmp_dir}/checksums.txt"

expected_checksum=$(grep " ${asset}$" "${tmp_dir}/checksums.txt" | awk '{print $1}')
if [ -z "${expected_checksum}" ]; then
  err "Checksum not found for ${asset} in checksums.txt"
fi

echo "Downloading ${asset}..."
curl --retry 3 -fL "${base_url}/${asset}" -o "${tmp_dir}/${asset}"

actual_checksum=$(shasum -a 256 "${tmp_dir}/${asset}" | awk '{print $1}')
if [ "${expected_checksum}" != "${actual_checksum}" ]; then
  err "Checksum mismatch: expected ${expected_checksum}, got ${actual_checksum}"
fi

tar -xzf "${tmp_dir}/${asset}" -C "${tmp_dir}"

if [ ! -f "${tmp_dir}/${BIN_NAME}" ]; then
  err "Binary not found after extraction."
fi

mkdir -p "${BIN_DIR}"

if [ -f "${BIN_DIR}/${BIN_NAME}" ]; then
  cp "${BIN_DIR}/${BIN_NAME}" "${BIN_DIR}/${BIN_NAME}.bak"
  echo "Existing binary backed up to ${BIN_DIR}/${BIN_NAME}.bak"
fi

install -m 755 "${tmp_dir}/${BIN_NAME}" "${BIN_DIR}/${BIN_NAME}"

echo "Installed ${BIN_NAME} to ${BIN_DIR}/${BIN_NAME}"

if [[ ":${PATH}:" != *":${BIN_DIR}:"* ]]; then
  echo "WARNING: ${BIN_DIR} is not in your PATH."
  echo "Add it to your PATH with:"
  echo "  export PATH=\"${BIN_DIR}:\$PATH\""
fi

echo "Done! Run '${BIN_NAME} --version' to verify."
