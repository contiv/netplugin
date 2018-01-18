#!/bin/bash

set -euo pipefail

$GOPATH/bin/modelgen ./ ./
gofmt -s -w *.go
gofmt -s -w client/*.go
