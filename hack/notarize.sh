#!/usr/bin/env bash
set -eu

# Notarization
# ============
# This script uploads binaries to Apple to get the code signatures trusted. Like the codesign script,
# it only makes sense to do this conditionally.

FILE="${1:?missing file argument}"
NAME="$(basename "$FILE" ".tar.gz")"
[ "$NAME" == "redskyctl-darwin-amd64" ] || { echo >&2 "skipping notarization for file=[$1]"; exit; }
command -v ditto >/dev/null 2>&1 || { echo >&2 "skipping notarization, ditto not present"; exit; }
command -v xcrun >/dev/null 2>&1 || { echo >&2 "skipping notarization, xcrun not present"; exit; }
command -v jq >/dev/null 2>&1 || { echo >&2 "skipping notarization, jq not present"; exit; }
[ -n "${AC_USERNAME:-}" ] || { echo >&2 "skipping notarization, no credentials"; exit; }
[ -n "${AC_PASSWORD:-}" ] || { echo >&2 "skipping notarization, no credentials"; exit; }

# Create a temporary location to perform notarization
WORKDIR="$(mktemp -d)"
function removeWorkdir()
{
  rm -rf "$WORKDIR"
}
trap removeWorkdir EXIT

# Re-archive as a Zip
mkdir "$WORKDIR/$NAME"
tar -xf "$FILE" -C "$WORKDIR/$NAME"
ditto -c -k "$WORKDIR/$NAME" "$WORKDIR/$NAME.zip"

# Helper functions
doNotarizeApp() {
  xcrun altool --notarize-app --file "$1" --primary-bundle-id "dev.redskyops.redskyctl" \
    -u "$AC_USERNAME" -p "@env:AC_PASSWORD" --output-format json | jq '."notarization-upload"'
}
doNotarizeInfo() {
  xcrun altool --notarization-info "$1" \
    -u "$AC_USERNAME" -p "@env:AC_PASSWORD" --output-format json | jq '."notarization-info"'
}

# Submit the Zip file for notarization, retain the request identifier
REQUEST_UUID="$(doNotarizeApp "$WORKDIR/$NAME.zip" | jq -r '.RequestUUID')"
[ "${REQUEST_UUID:-null}" != "null" ] || { echo >&2 "notarization request was not submitted"; exit 1; }
echo >&2 "notarization request submitted id=$REQUEST_UUID"

# Wait for a result
SLEEP_TIME=10
while true; do
  sleep $SLEEP_TIME
  INFO=$(doNotarizeInfo "$REQUEST_UUID")
  case "$(echo "$INFO" | jq -r '.Status')" in
    "in progress")
      SLEEP_TIME=5
      ;;
    "success")
      exit
      ;;
    "invalid")
      LOG_FILE_URL="$(echo "$INFO" | jq -r '.LogFileURL')"
      LOG="$(curl --silent "$LOG_FILE_URL")"
      echo >&2 "notarization failed: $(echo "$LOG" | jq -r '.statusSummary')"
      echo >&2 "more details: $LOG_FILE_URL"
      exit 1
      ;;
  esac
done
