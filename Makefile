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
	CONTIV_ENV=$(CONTIV_ENV) CONTIV_NODES=$(CONTIV_NODES) vagrant up

clean-demo:
	CONTIV_NODES=$(CONTIV_NODES) vagrant destroy -f

unit-test: build
	go test -v github.com/contiv/netplugin/drivers  \
		github.com/contiv/netplugin/plugin          \
		github.com/contiv/netplugin/netutils        \
		github.com/contiv/netplugin/gstate          \


system-test: build
	go test -v github.com/contiv/netplugin/systemtests
