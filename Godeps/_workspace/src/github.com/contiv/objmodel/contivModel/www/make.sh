#!/bin/sh
OS=`uname`
if [ $OS != "Darwin" ]; then
	echo "This can run only on OSX"
	exit 1
fi

# Install all bower components
bower install

# Install npm modules
npm install

# Create th ebundle
webpack
