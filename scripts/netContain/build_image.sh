#!/bin/bash

BUILD_ENV_IMAGE_NAME="contiv-netplugin-build"
TEMPORARY_CONTAINER_NAME="contiv-netplugin-build-temporary"
FINAL_IMAGE_NAME="contivbase"

NETPLUGIN_BINARIES="netplugin netmaster netctl contivk8s"

function get_image_id() {
	docker inspect --format '{{.ID}}' $1
	return $?
}

function build_image_from_path() {
	set +e
	old_image=$(get_image_id $2)
	set -e
	if [ "$?" -ne 0 ]; then
		old_image=
	fi

	echo "Building image $2 from $1 ..."
	docker build -t $2 --force-rm $1
	if [ "$?" -ne 0 ]; then
		echo "Image $2 was not built properly"
		return 1
	fi

	new_image=$(get_image_id $2)
	if [ "$old_image" != "" ] && [ "$old_image" != "$new_image" ]; then
		docker rmi $old_image >/dev/null 2>&1 || true
	fi

	echo "Image $2 has been built"
	return 0
}

set -x
set -e

# ensure this script wasn't called from the directory where this script
# lives; it should be called from the repository's top level
script_dir="$(dirname -- "$0")"
if [ "$script_dir" == "." ]; then
	echo "This script must be called from the top level of the repository"
	exit 1
fi

rm -rf scripts/netContain/bin
mkdir scripts/netContain/bin

build_image_from_path . $BUILD_ENV_IMAGE_NAME
if [ "$?" -ne 0 ]; then
	echo "Failed to build the "
	exit 1
fi

docker run --name=$TEMPORARY_CONTAINER_NAME $BUILD_ENV_IMAGE_NAME || true

echo "Copying the Contiv binaries..."
for f in $NETPLUGIN_BINARIES; do
	docker cp $TEMPORARY_CONTAINER_NAME:/go/bin/$f scripts/netContain/bin/
done

docker rm -fv $TEMPORARY_CONTAINER_NAME

echo "Building the final Docker image..."
build_image_from_path scripts/netContain contivbase
if [ "$?" -ne 0 ]; then
	echo "Failed to build the final Docker image"
	exit 1
fi

rm -rf scripts/netContain/bin
