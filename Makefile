all: build unit-test system-test

build:
	go get ./...
	go install -v -installsuffix=netplugin ./...

clean:
	go clean -i -r -v ./...

unit-test: build
	go test -v github.com/contiv/netplugin/drivers  \
		github.com/contiv/netplugin/plugin          \
		github.com/contiv/netplugin/netutils        \
		github.com/contiv/netplugin/gstate          \


system-test:
