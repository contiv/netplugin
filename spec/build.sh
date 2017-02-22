#!/bin/bash

set -euo pipefail

docker build -t contiv/spec .
cid=$(docker run -itd contiv/spec)
docker cp ${cid}:/contiv/contiv.html .
docker rm -fv ${cid}
