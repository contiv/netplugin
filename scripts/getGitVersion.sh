#!/bin/bash

set -euo pipefail

BUILD_VERSION=${BUILD_VERSION:-}
NIGHTLY_RELEASE=${NIGHTLY_RELEASE:-}

# calculate version
if command -v git &>/dev/null && git rev-parse &>/dev/null; then
	VERSION=$(git describe --abbrev=7 --dirty=-unsupported --tags --always 2>/dev/null || echo unknown)
else
	echo >&2 'error: unable to determine the git revision'
	exit 1
fi

# BUILD_VERSION overrides the git calculated version
if [ -n "$BUILD_VERSION" ]; then
	VERSION=$BUILD_VERSION
fi

if [ -z "$NIGHTLY_RELEASE" ]; then
	VERSION="$VERSION"
else
	BUILD_TIME=$(date -u +%m-%d-%Y.%H-%M-%S.UTC)
	VERSION="$VERSION-$BUILD_TIME"
fi

echo $VERSION
