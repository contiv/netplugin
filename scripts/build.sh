#!/bin/bash

set -euo pipefail

# build and install the code
bash generate.sh
go install ./ ./client/

# update the docs based on the latest code
pushd spec
make docs
popd
