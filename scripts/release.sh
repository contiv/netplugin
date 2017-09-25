#!/bin/bash
# Assumes following variables to be defined:
#  OLD_VERSION - previous version against which to create changelog
#  BUILD_VERSION - new version being released
#  GITHUB_USER - contiv
#  GITHUB_TOKEN - your github token
#  USE_RELEASE - if 0 or not set, will make a pre-release

if [ -z "$(which github-release)" ]; then
	echo "Please install github-release before running this script"
	echo "You may download a release from https://github.com/aktau/github-release/releases or run 'go get github.com/aktau/github-release' if you have Go installed"
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

if [ "$USE_RELEASE" != "1" ]; then
	echo "Making a pre-release.."
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
( (github-release -v release $pre_release -r netplugin -t $BUILD_VERSION -d "**Changelog**<br/>$changelog")) || exit 1
