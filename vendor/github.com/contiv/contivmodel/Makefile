
.PHONY: all build modelgen

all: build

build: modelgen
	@bash ./scripts/build.sh

modelgen:
	@if [ -z "`which modelgen`" ]; then go get -v github.com/contiv/modelgen; fi
