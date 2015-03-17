.PHONY: all build clean default system-test unit-test

TO_BUILD := ./ ./netdcli/ 

all: build unit-test system-test

default: build

build:
	./scripts/checks
	go get -d $(TO_BUILD)
	go install -v $(TO_BUILD)

clean:
	go clean -i -v ./...

demo: build
	CONTIV_ENV="$(CONTIV_ENV)" CONTIV_NODES=$(CONTIV_NODES) vagrant up

clean-demo:
	CONTIV_NODES=$(CONTIV_NODES) vagrant destroy -f

unit-test: build
	./scripts/unittests -vagrant

system-test: build
	go test -v -run "sanity" github.com/contiv/netplugin/systemtests/singlehost 
	go test --timeout 20m -v -run "sanity" github.com/contiv/netplugin/systemtests/twohosts

regress-test: build
	go test -v -run "regress" github.com/contiv/netplugin/systemtests/singlehost 
	go test --timeout 60m -v -run "regress" github.com/contiv/netplugin/systemtests/twohosts
