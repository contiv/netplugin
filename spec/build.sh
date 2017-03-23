#!/bin/bash

set -euo pipefail

IMAGE_NAME="contiv/raml2html"

mkdir -p docs

docker build -t $IMAGE_NAME .

echo "generating netmaster docs"
docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/contiv \
       $IMAGE_NAME -i contiv.raml -o docs/contiv.html

echo "generating auth_proxy docs"
docker run --rm \
       -u $(id -u):$(id -g) \
       -v $(pwd):/contiv \
       $IMAGE_NAME -i auth_proxy.raml -o docs/auth_proxy.html

