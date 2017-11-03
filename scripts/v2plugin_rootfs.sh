#!/bin/bash

# Script to create the docker v2 plugin
# run this script from contiv/netplugin directory

# requires NETPLUGIN_CONTAINER_TAG for contivrootfs image
# requires CONTIV_V2PLUGIN_NAME, the Network Driver name for requests to
#   dockerd, should look like contiv/v2plugin:$NETPLUGIN_CONTAINER_TAG
# requires TAR_FILE to point to the netplugin binaries
# requires V2PLUGIN_TAR_FILENAME to point to the v2plugin archive

set -euxo pipefail

echo "Creating rootfs for v2plugin: ${CONTIV_V2PLUGIN_NAME}"


# config.json is docker's runtime configuration for the container
# delete comments and replace placeholder with ${CONTIV_V2PLUGIN_NAME}
sed '/##/d;s/__CONTIV_V2PLUGIN_NAME__/${CONTIV_V2PLUGIN_NAME}/' \
    install/v2plugin/config.template > install/v2plugin/config.json

# copy over binaries
cp ${TAR_FILE} install/v2plugin/

DOCKER_IMAGE=contivrootfs:${NETPLUGIN_CONTAINER_TAG}
docker build -t ${DOCKER_IMAGE} \
    --build-arg TAR_FILE=$(basename "${TAR_FILE}") install/v2plugin

rm install/v2plugin/${TAR_FILE}  # this was a copy of netplugin archive

# creates a ready to run container but doesn't run it
id=$(docker create $DOCKER_IMAGE true)

# create the rootfs archive based on the created container contents
docker export "${id}" > install/v2plugin/${V2PLUGIN_TAR_FILENAME}

# clean up created container
docker rm -vf "${id}"

echo netplugin\'s docker plugin rootfs is archived at install/v2plugin/${V2PLUGIN_TAR_FILENAME}
