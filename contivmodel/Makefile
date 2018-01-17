
.PHONY: all build godep modelgen systemtests

all: build

build: modelgen
	@bash ./scripts/build.sh

godep:
	godep save ./...

modelgen:
	@go get -u -v github.com/contiv/modelgen

# systemtest runs all of the systemtests
systemtests:
	go test -v -timeout 5m ./systemtests -check.v
