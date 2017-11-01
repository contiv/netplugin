#!/bin/bash

set -euo pipefail

PKG_DIRS=$*

docker build -f Dockerfile-check -t contiv/netplugin-checks .

# create container, stdout is the created container id
container_id=$(docker create --name netplugin-checks \
    contiv/netplugin-checks)

# when there is an exit, remove the container
function remove_container {
    docker rm ${container_id}
}
trap remove_container EXIT

NETPLUGIN_DIR=/go/src/github.com/contiv/netplugin
# copy Makefile and go packages to be checked
docker cp Makefile ${container_id}:${NETPLUGIN_DIR}/
for pkg in ${PKG_DIRS}; do
    docker cp $pkg ${container_id}:${NETPLUGIN_DIR}/
done

# run the checks
docker start --attach ${container_id}
