#!/bin/bash

set -euo pipefail

# build and install the code
bash generate.sh
go install ./ ./client/

# regenerate netmaster.raml
docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/files \
       -w /files/spec \
       ruby:2.4.0-slim /usr/local/bin/ruby contivModel2raml.rb
mv spec/netmaster.raml ./spec/contiv/libraries/netmaster.raml

# run the raml2html tool to generate docs under spec/docs
pushd spec
make docs
mkdir -p docs
mv contiv.html docs/
popd

# because we have to do some tidying up of the output HTML and it requires some
# external dependencies, we use a small Docker image to do it.
# this image uses the same ruby:2.4.0-slim base as above.
IMAGE_NAME="contiv_api_documentation_cleanup"

if [[ "$(docker images -q $IMAGE_NAME:latest 2>/dev/null)" == "" ]]; then
    docker build -t $IMAGE_NAME -f spec/Dockerfile.cleanup .
fi

docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/files \
       -w /files/spec \
       $IMAGE_NAME /usr/local/bin/ruby cleanup.rb
