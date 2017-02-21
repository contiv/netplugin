
.PHONY: all build

all: build

modelgen:
	@if [ -z "`which modelgen`" ]; then go get -v github.com/contiv/modelgen; fi

build: modelgen
	bash generate.sh
	go install ./ ./client/
	docker run --rm -u $(shell id -u):$(shell id -g) -v $(PWD):/files -w /files/spec ruby:2.4.0-slim /usr/local/bin/ruby contivModel2raml.rb
	mv spec/netmaster.raml ./spec/contiv/libraries/netmaster.raml
