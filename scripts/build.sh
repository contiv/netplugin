#!/bin/bash

BUILD_TIME=$(date -u +%m-%d-%Y.%H-%M-%S.UTC)
VERSION=$(cat version/CURRENT_VERSION | tr -d '\n')
PKG_NAME=github.com/contiv/netplugin/version

# BUILD_VERSION overrides the version from CURRENT_VERSION
if [ -n "$BUILD_VERSION" ]; then
	VERSION=$BUILD_VERSION
fi

if [ -z "$USE_RELEASE" ]; then
	BUILD_VERSION="$VERSION-$BUILD_TIME"
else
	BUILD_VERSION="$VERSION"
fi

if command -v git &>/dev/null && git rev-parse &>/dev/null; then
	GIT_COMMIT=$(git rev-parse --short HEAD)
	if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
		GIT_COMMIT="$GIT_COMMIT-unsupported"
	fi
else
	echo >&2 'error: unable to determine the git revision'
	exit 1
fi

echo $BUILD_VERSION >$VERSION_FILE

GOGC=1500 go install \
	-ldflags "-X $PKG_NAME.version=$BUILD_VERSION \
	-X $PKG_NAME.buildTime=$BUILD_TIME \
	-X $PKG_NAME.gitCommit=$GIT_COMMIT \
	-s -w" \
	-v $TO_BUILD
