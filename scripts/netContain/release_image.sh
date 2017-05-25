#!/bin/bash

set -euo pipefail

contiv_version=""
image_name="contiv/netplugin"
image_tag=""

function usage() {
	echo "$0: push the target netplugin release to Docker Hub as an image"
	echo ""
	echo "Note: the image is uploaded as whatever user you are logged in as."
	echo "Please \`docker login\` beforehand and ensure your user has access to the \"contiv\" organization."
	echo ""
	echo "Usage:"
	echo "./release_image.sh -v <contiv version> [-i <image name>] [-t <image tag>]"
	echo "Example: ./release_image.sh -v 1.2.3"
	echo "Released versions are available from https://github.com/contiv/netplugin/releases"
	echo "Default values are:"
	echo "  Image name: \"contiv/netplugin\""
	echo "  Image tag: value of -v switch"
	exit 1
}

while getopts ":v:i:t:" opt; do
	case $opt in
		v)
			contiv_version=$OPTARG
			;;
		i)
			image_name=$OPTARG
			;;
		t)
			image_tag=$OPTARG
			;;
		:)
			echo "An argument required for $OPTARG was not passed"
			usage
			;;
		?)
			usage
			;;
	esac
done

if [ "$contiv_version" = "" ]; then
	usage
fi

if [ "$image_tag" = "" ]; then
	image_tag=$contiv_version
fi

filename="netplugin-$contiv_version.tar.bz2"
image="$image_name:$image_tag"

cd -P -- "$(dirname -- "$0")"

# empty this directory or tar will fail to extract
rm -rf bin && mkdir bin

curl -L -o $filename https://github.com/contiv/netplugin/releases/download/$contiv_version/$filename
tar xvfj $filename -C bin

# remove the contrib directory, we don't need it in the image
rm -rf bin/contrib

docker build . -t $image -t ${image_name}:latest
docker push $image

echo ""
echo "SUCCESS: Pushed contiv version $contiv_version to $image"
