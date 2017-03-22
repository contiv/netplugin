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

# RAML doesn't currently support trailing slashes so we add them manually here

# altering the HTML requires a gem called Nokogiri
# create a tiny docker image so we don't have to reinstall Nokogiri every time
IMAGE_NAME="raml_trailing_slashes"

if [[ "$(docker images -q $IMAGE_NAME:latest 2>/dev/null)" == "" ]]; then
    docker build -t $IMAGE_NAME -f spec/Dockerfile.raml_trailing_slashes .
fi

docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/files \
       -w /files/spec \
       $IMAGE_NAME /usr/local/bin/ruby raml_trailing_slashes.rb
