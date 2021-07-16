#!/usr/bin/env bash
set -eu

# Notarization
# ============
# This script combines two painful concepts. Apple Notarization and GoReleaser Signing.

FILE="${1:?missing file argument}"
OUTPUT="${2:?missing output argument}"

# This script MUST produce an output file or fail.
case "$(basename "$FILE")" in
  stormforge-darwin-*)
    # If there are no credentials, just produce an empty file (otherwise fall through)
    if [ -z "${AC_USERNAME:-}" ] || [ -z "${AC_PASSWORD:-}" ] ; then
      echo "Not empty" > "${OUTPUT}"
      exit
    fi
    ;;
  "checksums.txt")
    # Sign the checksums using GPG (mimic the default GoReleaser behavior)
    gpg --output "${OUTPUT}" --detach-sign "${FILE}"
    exit
    ;;
  *)
    # Just create an empty file to upload to the release
    echo "Not empty" > "${OUTPUT}"
    exit
    ;;
esac

# Verify we can actually do something
command -v ditto >/dev/null 2>&1 || { echo >&2 "notarization failed, ditto not present"; exit 1; }
command -v xcrun >/dev/null 2>&1 || { echo >&2 "notarization failed, xcrun not present"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo >&2 "notarization failed, jq not present"; exit 1; }

# Create a temporary location to perform notarization
NAME="$(basename "$FILE" ".tar.gz")"
WORKDIR="$(mktemp -d)"
function removeWorkdir()
{
  rm -rf "$WORKDIR"
}
trap removeWorkdir EXIT

# Re-archive as a Zip (we cannot just do code signing here and re-pack the tarball without also re-computing `checksums.txt`)
mkdir "$WORKDIR/$NAME"
tar -xf "$FILE" -C "$WORKDIR/$NAME"
ditto -c -k "$WORKDIR/$NAME" "$WORKDIR/$NAME.zip"

# Helper functions
doNotarizeApp() {
  xcrun altool --notarize-app --file "$1" --primary-bundle-id "io.stormforge.optimize.cli" \
    -u "$AC_USERNAME" -p "@env:AC_PASSWORD" --output-format json | jq '."notarization-upload"'
}
doNotarizeInfo() {
  xcrun altool --notarization-info "$1" \
    -u "$AC_USERNAME" -p "@env:AC_PASSWORD" --output-format json | jq '."notarization-info"'
}

# Submit the Zip file for notarization, retain the request identifier
REQUEST_UUID="$(doNotarizeApp "$WORKDIR/$NAME.zip" | jq -r '.RequestUUID')"
[ "${REQUEST_UUID:-null}" != "null" ] || { echo >&2 "notarization failed, request was not submitted"; exit 1; }
echo "${REQUEST_UUID}" >> "${OUTPUT}"

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
