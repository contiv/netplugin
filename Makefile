
.PHONY: all build godep modelgen systemtests

all: build

build: modelgen
	@bash ./scripts/build.sh

godep:
	godep save ./...

modelgen:
	@if [ -z "`which modelgen`" ]; then go get -v github.com/contiv/modelgen; fi

# systemtest runs all of the systemtests
systemtests:
	go test -v -timeout 5m ./systemtests -check.v
