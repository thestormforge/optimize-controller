#!/usr/bin/env bash
set -eu

# Code Signing
# ============
# This script facilitates code signing of binaries for macOS. This script conditionally signs a binary
# under the following conditions:
# 1. The binary path contains "darwin" (this prevents duplicating GoReleaser build configurations)
# 2. The `security` and `codesign` programs are available (this allows the script to be run on Linux)
# 3. The P12 key pair used as the signing identity is available (allows the script to be run on non-release builds)

# Verify, but exit normally to ensure the conditional nature of the checks
FILE="${1:?missing file argument}"
[[ "$(basename "$(dirname "$FILE")")" == stormforge_darwin_* ]] || { echo >&2 "skipping code signing on file '$1'"; exit; }
command -v security >/dev/null 2>&1 || { echo >&2 "skipping code signing, security not present"; exit; }
command -v codesign >/dev/null 2>&1 || { echo >&2 "skipping code signing, codesign not present"; exit; }
[ -n "${AC_IDENTITY_P12:-}" ] || { echo >&2 "skipping code signing, no signing identity"; exit; }

# Create a temporary location for the keychain
AC_IDENTITY_PASSPHRASE="${AC_IDENTITY_PASSPHRASE:-$AC_PASSWORD}"
KEYCHAIN_PASSWORD="$(openssl rand -base64 30)"
KEYCHAIN="$(mktemp -d)/codesigning.keychain-db"
removeKeychain() {
  rm -rf "$(dirname "$KEYCHAIN")"
}
trap removeKeychain EXIT

# Create an unlocked keychain for signing
security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN"
security set-keychain-settings "$KEYCHAIN" # Unset keychain settings (disable lock)
security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN"

# Import the signing identity to the temporary keychain
security import <(printenv AC_IDENTITY_P12 | base64 -D) -f pkcs12 -k "$KEYCHAIN" -P "$AC_IDENTITY_PASSPHRASE" -T "$(command -v codesign)" >/dev/null
IDENTITY="$(security find-identity -v -p "codesigning" "$KEYCHAIN" | grep "1)" | awk '{print $2}')"
[ -n "${IDENTITY:-}" ] || { echo >&2 "failed to import signing identity"; exit 1; }
echo >&2 "signing with id=$IDENTITY"

# Update the keychain search list so codesign can use it
security list-keychains -d user -s "$KEYCHAIN"
restoreKeychain() {
  security list-keychains -d user -s login.keychain
}
trap restoreKeychain EXIT

# Actually sign the supplied file
codesign --keychain "$KEYCHAIN" --sign "$IDENTITY" --force --verbose --timestamp --options "runtime" "$FILE"
