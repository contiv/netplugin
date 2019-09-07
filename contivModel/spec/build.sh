#!/bin/bash

set -euo pipefail

mkdir -p docs

#
# regenerate libraries/netmaster.raml which holds the type netmaster type data
#
docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd)/../:/files \
       -w /files/spec \
       ruby:2.4.0-slim /usr/local/bin/ruby generate_raml.rb ./netmaster/libraries/netmaster.raml

#
# convert the raml into HTML output
#
RAML_IMAGE_NAME="contiv/raml2html"

if [[ "$(docker images -q $RAML_IMAGE_NAME:latest 2>/dev/null)" == "" ]]; then
    docker build -t $RAML_IMAGE_NAME .
fi

echo "generating netmaster docs"
docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/contiv \
       $RAML_IMAGE_NAME -i netmaster.raml -o docs/contiv.html

echo "generating auth_proxy docs"
docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/contiv \
       $RAML_IMAGE_NAME -i auth_proxy.raml -o docs/auth_proxy.html

#
# because we have to do some tidying up of the output HTML and it requires some
# external dependencies, we use a small Docker image to do it.
# this image uses the same ruby:2.4.0-slim base as above.
#
CLEANUP_IMAGE_NAME="contiv/api_documentation_cleanup"

if [[ "$(docker images -q $CLEANUP_IMAGE_NAME:latest 2>/dev/null)" == "" ]]; then
    docker build -t $CLEANUP_IMAGE_NAME -f Dockerfile.cleanup .
fi

echo "Cleaning up HTML output"
docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/files \
       -w /files \
       $CLEANUP_IMAGE_NAME /usr/local/bin/ruby cleanup.rb
