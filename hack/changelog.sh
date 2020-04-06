#!/usr/bin/env sh

echo "## Changelog"

# This script generates the _draft_ for release notes. The point here is not to regurgatate the commit
# history since the last release, but rather to provide a user focused summary of what changed.

# The approach is based on advice from [Keep A Changelog](https://keepachangelog.com/); however given
# that the project already uses GitHub releases, it was decided that eliminating duplication and
# the SCM overhead of maintianing a separate file outweighted the drawbacks listed in their FAQ.

# TODO Rename sections to match Keep A Changelog? We should make the change retroactively as well
# "New Features" = "Added"
# "Improvements" = "Changed"
# "" = "Deprecated"
# "Breaking Changes" = "Removed"
# "Bug Fixes" = "Fixed"
# "" = "Security"
# Use "-" instead of "*"

# TODO Are there some commits we actually do want to include automatically? This is just the draft notes...
# Looking only at detail messages from merge commits matching some branches (e.g. "refs/branches/fix/*") eliminates most of the noise
# Some commit messages (e.g. "Upgraded X to X") always make it into the changelog

cat << EOF

### ✨ New Features

*

### 🏗 Improvements

*

### 🐛 Bug Fixes

*

### 🛑 Breaking Changes

*

EOF

# Include the Docker images by digest in the release notes.
echo "## 🏷 Image Tags"
echo
for img in "${IMG}" "${SETUPTOOLS_IMG}" "${REDSKYCTL_IMG}"; do
  if [ -n "${img}" ]; then
    echo "* \`${img%%:*}@$(docker images --no-trunc --quiet "${img}")\`"
  fi
done

