#!/bin/bash

set -euo pipefail

REPOSITORY="contivovs"
IMAGE=$REPOSITORY:${NETPLUGIN_CONTAINER_TAG:-latest}

function get_image_id() {
	docker inspect --format '{{.ID}}' $IMAGE || :
}

old_image=$(get_image_id)

cd scripts/ovscontainer

docker build -t $IMAGE .

new_image=$(get_image_id)

if [ "$old_image" != "" ] && [ "$old_image" != "$new_image" ]; then
	echo Removing old image $old_image
	docker rmi -f $old_image >/dev/null 2>&1 || true
fi
