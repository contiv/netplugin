#!/bin/bash

set -euxo pipefail

GIT_COMMIT=$(./scripts/getGitCommit.sh)

PKG_NAME=github.com/contiv/netplugin/version
GOGC=1500 CGO_ENABLED=0 go install -v \
	-a -installsuffix cgo \
	-ldflags "-X $PKG_NAME.version=$BUILD_VERSION \
	-X $PKG_NAME.buildTime=$(date -u +%m-%d-%Y.%H-%M-%S.UTC) \
	-X $PKG_NAME.gitCommit=$GIT_COMMIT \
	-s -w -d" -pkgdir /tmp/foo-cgo \
	$TO_BUILD
