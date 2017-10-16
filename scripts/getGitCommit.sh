#!/bin/bash

set -euo pipefail

if command -v git &>/dev/null && git rev-parse &>/dev/null; then
    GIT_COMMIT=$(git rev-parse --short HEAD)
    if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
        GIT_COMMIT="$GIT_COMMIT-unsupported"
    fi
    echo $GIT_COMMIT
    exit 0
fi
echo >&2 'error: unable to determine the git revision'
exit 1
