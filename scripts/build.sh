#!/bin/bash

set -euxo pipefail

GIT_COMMIT=$(./scripts/getGitCommit.sh)

# TODO(chrisplo): remove when contiv/install no longer needs
echo $BUILD_VERSION >netplugin-version

PKG_NAME=github.com/contiv/netplugin/version
GOGC=1500 go install -v \
	-ldflags "-X $PKG_NAME.version=$BUILD_VERSION \
	-X $PKG_NAME.buildTime=$(date -u +%m-%d-%Y.%H-%M-%S.UTC) \
	-X $PKG_NAME.gitCommit=$GIT_COMMIT \
	-s -w" \
	$TO_BUILD
