
.PHONY: all build godep modelgen

all: build

build: modelgen
	@bash ./scripts/build.sh

godep:
	godep save ./...

modelgen:
	@if [ -z "`which modelgen`" ]; then go get -v github.com/contiv/modelgen; fi
