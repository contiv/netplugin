#!/bin/bash

# Launch a marathon job

USAGE="Usage: $0 <marathon-json-file>"

if [ $# -ne 1 ]; then
    echo $USAGE
    exit 1
fi

curl -X POST -H "Content-Type: application/json" http://localhost:8080/v2/apps -d@$1
