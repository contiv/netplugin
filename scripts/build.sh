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
cd spec
make docs
mkdir -p docs
mv contiv.html docs/
