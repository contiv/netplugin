all: build unit-test system-test

build:
	go install -v

unit-test:
	go test -v github.com/mapuri/netplugin/drivers

system-test:
