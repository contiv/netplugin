#!/bin/bash

set -euo pipefail

# build and install the code
bash generate.sh
go install ./ ./client/

# update the docs based on the latest code

# NOTE: disabled on 2017/06/27 due to a breakage in the RAML Dockerfile.  it's complaining
#       about a xhr2 package being missing or something.  since we're not even using the
#       generated documentation, disabling it is the best option for now.

# pushd spec
# make docs
# popd
