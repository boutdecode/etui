#!/usr/bin/env bash
# Tests for install.sh (no external dependency). Run: bash install_test.sh
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=install.sh
source "$here/install.sh"
set +e

pass=0
fail=0
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

ok() { pass=$((pass + 1)); }
ko() { fail=$((fail + 1)); printf 'FAIL: %s\n' "$*"; }

# assert_eq <name> <expected> <actual>
assert_eq() { if [[ "$2" == "$3" ]]; then ok; else ko "$1: expected '$2', got '$3'"; fi; }

# assert_fails <name> <command...>: command (run in a subshell) must exit non-zero.
assert_fails() {
  local name="$1"
  shift
  if ( "$@" ) >/dev/null 2>&1; then ko "$name: expected failure"; else ok; fi
}

# Architecture mapping
assert_eq "arch x86_64" amd64 "$(ETUI_TEST_UNAME_M=x86_64 detect_arch)"
assert_eq "arch aarch64" arm64 "$(ETUI_TEST_UNAME_M=aarch64 detect_arch)"
assert_eq "arch arm64" arm64 "$(ETUI_TEST_UNAME_M=arm64 detect_arch)"
ETUI_TEST_UNAME_M=riscv64 assert_fails "arch unsupported" detect_arch

# OS mapping / unsupported platform
assert_eq "os Linux" linux "$(ETUI_TEST_UNAME_S=Linux detect_os)"
assert_eq "os Darwin" darwin "$(ETUI_TEST_UNAME_S=Darwin detect_os)"
ETUI_TEST_UNAME_S=FreeBSD assert_fails "os unsupported" detect_os

# Version normalization
assert_eq "version v1.2.3" 1.2.3 "$(normalize_version v1.2.3)"
assert_eq "version 1.2.3" 1.2.3 "$(normalize_version 1.2.3)"
assert_eq "version rc" 0.0.1-rc1 "$(normalize_version v0.0.1-rc1)"
assert_fails "version invalid" normalize_version "latest"
assert_fails "version injection" normalize_version "1.2.3/../x"

# Checksums
archive="$work/etui_1.0.0_linux_amd64.tar.gz"
echo "payload" >"$archive"
good=$(sha256_of "$archive")
echo "$good  etui_1.0.0_linux_amd64.tar.gz" >"$work/good.txt"
echo "$(printf '0%.0s' {1..64})  etui_1.0.0_linux_amd64.tar.gz" >"$work/bad.txt"
echo "$good  other.tar.gz" >"$work/missing.txt"
if ( verify_checksum "$archive" "$work/good.txt" ) >/dev/null 2>&1; then ok; else ko "checksum valid"; fi
assert_fails "checksum mismatch" verify_checksum "$archive" "$work/bad.txt"
assert_fails "checksum missing entry" verify_checksum "$archive" "$work/missing.txt"

# Non-HTTPS download refused
assert_fails "download http" download "http://example.com/x" "$work/x"

# Uninstall
d="$work/bin"
mkdir -p "$d"
out=$(main --uninstall --dir "$d" 2>&1)
case "$out" in *"Nothing to remove"*) ok ;; *) ko "uninstall without binary: $out" ;; esac
touch "$d/etui"
out=$(main --uninstall --dir "$d" 2>&1)
if [[ ! -e "$d/etui" && "$out" == *Removed* ]]; then ok; else ko "uninstall with binary: $out"; fi
touch "$d/etui"
ETUI_INSTALL_DIR="$d" main --uninstall >/dev/null 2>&1
if [[ ! -e "$d/etui" ]]; then ok; else ko "uninstall via ETUI_INSTALL_DIR"; fi

# Argument errors
assert_fails "unknown argument" main --bogus
assert_fails "--version without value" main --version
assert_fails "--dir without value" main --dir

# Help
out=$(main --help 2>&1)
case "$out" in *Usage*) ok ;; *) ko "help" ;; esac

printf '%d passed, %d failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]]
