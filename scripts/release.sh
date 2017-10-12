#!/bin/bash
# Assumes following variables to be defined:
#  OLD_VERSION - previous version against which to create changelog
#  BUILD_VERSION - new version being released
#  GITHUB_USER - contiv
#  GITHUB_TOKEN - your github token

if [ -z "$(which github-release)" ]; then
	echo "Please install github-release before running this script"
	echo "You may download a release from https://github.com/aktau/github-release/releases or run 'go get github.com/aktau/github-release' if you have Go installed"
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

if [ -z "$BUILD_VERSION" ]; then
	echo "A release requires BUILD_VERSION to be defined"
	exit 1
fi

if [ -z "$OLD_VERSION" ]; then
	echo "A release requires OLD_VERSION to be defined"
	exit 1
fi

if [ "$OLD_VERSION" != "none" ]; then
	comparison="$OLD_VERSION..HEAD"
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
( (github-release -v release --pre-release -r netplugin -t $BUILD_VERSION -d "**Changelog**<br/>$changelog") \
	&& (github-release -v upload -r netplugin -t $BUILD_VERSION -n $TAR_FILENAME -f $TAR_FILE \
		|| github-release -v delete -r netplugin -t $BUILD_VERSION)) || exit 1
