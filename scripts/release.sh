#!/bin/bash

set -euo pipefail

if [ -z "${VERSION-}" ]; then
	echo "VERSION needs to be defined to make a release"
	exit 1
fi

if [ -z "${TAR_FILENAME-}" ]; then
	echo "TAR_FILENAME needs to be defined to make a release"
	exit 1
fi

if [ ! -f "$TAR_FILE" ]; then
	echo "TAR_FILE ($TAR_FILE) doesn't exist"
	exit 1
fi

if [ -n "$USE_RELEASE" ]; then
	if [ -z "$OLD_VERSION" ]; then
		echo "A release requires OLD_VERSION to be defined"
		exit 1
	fi
	if [ "$OLD_VERSION" != "none" ]; then
		comparison="$OLD_VERSION..HEAD"
	fi
	pre_release=""
else
	latest_tag=$(git tag | egrep -v "^v" | grep UTC | sort -V | tail -1)

	comparison="$latest_tag..HEAD"
	echo "Making a pre-release..."
	pre_release="-p"
fi

if [ "$OLD_VERSION" != "none" ]; then
	changelog=$(git log $comparison --oneline --no-merges --reverse)

	if [ -z "$changelog" ]; then
		echo "No new changes to release!"
		exit 0
	fi
else
	changelog="don't forget to update the changelog"
fi

set -x
( (github-release -v release $pre_release -r netplugin -t $VERSION -d "**Changelog**<br/>$changelog") \
	&& (github-release -v upload -r netplugin -t $VERSION -n $TAR_FILENAME -f $TAR_FILE \
		|| github-release -v delete -r netplugin -t $VERSION)) || exit 1
