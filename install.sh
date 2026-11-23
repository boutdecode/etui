#!/usr/bin/env bash
# etui installer.
#
#   curl -fsSL https://github.com/boutdecode/etui/releases/latest/download/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --version v1.2.3
#   curl -fsSL .../install.sh | bash -s -- --uninstall
#
# Environment: ETUI_VERSION, ETUI_INSTALL_DIR (options take precedence).
set -euo pipefail

REPO="boutdecode/etui"
BIN="etui"
DEFAULT_DIR="${HOME:-}/.local/bin"

TMP_DIR=""

log() { printf '%s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$TMP_DIR" ]]; then rm -rf "$TMP_DIR"; fi
}

usage() {
  cat <<EOF
Usage: install.sh [options]

Options:
  --version <tag>   Install a specific version (v1.2.3 or 1.2.3). Default: latest.
  --dir <path>      Install directory. Default: ~/.local/bin.
  --uninstall       Remove ${BIN} from the install directory.
  -h, --help        Show this help.

Environment variables: ETUI_VERSION, ETUI_INSTALL_DIR (options take precedence).
EOF
}

# detect_os prints linux or darwin.
detect_os() {
  local os="${ETUI_TEST_UNAME_S:-$(uname -s)}"
  case "$os" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) die "unsupported operating system: $os (supported: Linux, macOS)" ;;
  esac
}

# detect_arch prints amd64 or arm64.
detect_arch() {
  local arch="${ETUI_TEST_UNAME_M:-$(uname -m)}"
  case "$arch" in
    x86_64 | amd64) echo amd64 ;;
    aarch64 | arm64) echo arm64 ;;
    *) die "unsupported architecture: $arch (supported: x86_64, aarch64/arm64)" ;;
  esac
}

# normalize_version prints the version without the leading "v".
normalize_version() {
  local v="${1#v}"
  [[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+.][0-9A-Za-z.+-]+)?$ ]] || die "invalid version: $1"
  echo "$v"
}

have() { command -v "$1" >/dev/null 2>&1; }

# download <url> <dest>: HTTPS only.
download() {
  local url="$1" dest="$2"
  [[ "$url" == https://* ]] || die "refusing non-HTTPS URL: $url"
  if have curl; then
    curl -fsSL --proto '=https' --tlsv1.2 -o "$dest" "$url" || die "download failed: $url"
  elif have wget; then
    wget -q --https-only -O "$dest" "$url" || die "download failed: $url"
  else
    die "curl or wget is required"
  fi
}

# latest_version prints the latest release tag (e.g. v1.2.3).
latest_version() {
  local url="https://github.com/${REPO}/releases/latest" final=""
  if have curl; then
    final=$(curl -fsSLI --proto '=https' --tlsv1.2 -o /dev/null -w '%{url_effective}' "$url") ||
      die "could not resolve the latest release"
  elif have wget; then
    final=$(wget --https-only --max-redirect=0 --server-response -qO /dev/null "$url" 2>&1 |
      sed -n 's/^ *[Ll]ocation: *//p' | tr -d '\r' | tail -n1) || true
  else
    die "curl or wget is required"
  fi
  local tag="${final##*/}"
  [[ "$final" == */releases/tag/* && -n "$tag" ]] || die "could not resolve the latest release (is there one published?)"
  echo "$tag"
}

# sha256_of <file> prints the hex digest.
sha256_of() {
  if have sha256sum; then
    sha256sum "$1" | awk '{print $1}'
  elif have shasum; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    die "sha256sum or shasum is required"
  fi
}

# verify_checksum <archive> <checksums.txt>: fails on mismatch or missing entry.
verify_checksum() {
  local file="$1" sums="$2" name expected actual
  name=$(basename "$file")
  expected=$(awk -v n="$name" '$2 == n || $2 == "*"n {print $1; exit}' "$sums")
  [[ -n "$expected" ]] || die "no checksum found for $name"
  actual=$(sha256_of "$file")
  [[ "$expected" == "$actual" ]] || die "checksum mismatch for $name (expected $expected, got $actual)"
}

do_uninstall() {
  local dir="$1" target="$1/$BIN"
  if [[ -e "$target" || -L "$target" ]]; then
    rm -f "$target"
    log "Removed $target"
  else
    log "Nothing to remove: $target not found"
  fi
}

do_install() {
  local version="$1" dir="$2" os arch tag name base
  os=$(detect_os)
  arch=$(detect_arch)
  if [[ -z "$version" ]]; then
    tag=$(latest_version)
  else
    tag="v$(normalize_version "$version")"
  fi
  version=$(normalize_version "$tag")
  name="${BIN}_${version}_${os}_${arch}.tar.gz"
  base="https://github.com/${REPO}/releases/download/${tag}"

  TMP_DIR=$(mktemp -d)
  trap cleanup EXIT

  log "Installing ${BIN} ${tag} (${os}/${arch}) to ${dir}"
  download "$base/$name" "$TMP_DIR/$name"
  download "$base/checksums.txt" "$TMP_DIR/checksums.txt"
  verify_checksum "$TMP_DIR/$name" "$TMP_DIR/checksums.txt"

  mkdir -p "$TMP_DIR/extract"
  tar -xzf "$TMP_DIR/$name" -C "$TMP_DIR/extract" "$BIN" || die "could not extract $BIN from $name"

  mkdir -p "$dir" || die "cannot create $dir"
  install -m 0755 "$TMP_DIR/extract/$BIN" "$dir/$BIN" || die "cannot install into $dir"

  "$dir/$BIN" --version || warn "installed, but '$dir/$BIN --version' failed"

  case ":${PATH}:" in
    *":${dir}:"*) ;;
    *)
      warn "$dir is not in your PATH. Add this line to your shell profile:"
      printf '  export PATH="%s:$PATH"\n' "$dir" >&2
      ;;
  esac
}

main() {
  local version="${ETUI_VERSION:-}" dir="${ETUI_INSTALL_DIR:-$DEFAULT_DIR}" uninstall=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        [[ $# -ge 2 ]] || die "--version requires a value"
        version="$2"
        shift 2
        ;;
      --version=*)
        version="${1#*=}"
        shift
        ;;
      --dir)
        [[ $# -ge 2 ]] || die "--dir requires a value"
        dir="$2"
        shift 2
        ;;
      --dir=*)
        dir="${1#*=}"
        shift
        ;;
      --uninstall)
        uninstall=1
        shift
        ;;
      -h | --help)
        usage
        return 0
        ;;
      *)
        usage >&2
        die "unknown argument: $1"
        ;;
    esac
  done

  [[ -n "$dir" ]] || die "install directory is empty (is HOME set?)"

  if [[ "$uninstall" == 1 ]]; then
    do_uninstall "$dir"
  else
    do_install "$version" "$dir"
  fi
}

# Run only when executed (or piped to bash), not when sourced by tests.
if [[ "${BASH_SOURCE[0]:-$0}" == "$0" ]]; then
  main "$@"
fi
