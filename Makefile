all: build unit-test system-test

build:
	go get ./...
	go install -v 

unit-test: build
	go test -v github.com/mapuri/netplugin/drivers

system-test:
