
.PHONY: all build

all: build

modelgen:
	@if [ -z "`which modelgen`" ]; then go get -v github.com/contiv/modelgen; fi

build: modelgen
	bash generate.sh
	go install ./ ./client/
