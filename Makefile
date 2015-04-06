.PHONY: all build clean default system-test unit-test

TO_BUILD := ./ ./netdcli/ ./mgmtfn/k8contivnet/
HOST_GOBIN := `which go | xargs dirname`
HOST_GOROOT := `go env GOROOT`

all: build unit-test system-test

default: build

build:
	./scripts/checks
	go get -d $(TO_BUILD)
	go install -v $(TO_BUILD)

clean:
	go clean -i -v ./...

# setting CONTIV_NODES=<number> while calling 'make demo' can be used to bring
# up a cluster of <number> nodes. By default <number> = 1
demo: build
	CONTIV_HOST_GOBIN=$(HOST_GOBIN) CONTIV_HOST_GOROOT=$(HOST_GOROOT) vagrant up

clean-demo:
	vagrant destroy -f

start-dockerdemo:
	scripts/dockerhost/start-dockerhosts

clean-dockerdemo:
	scripts/dockerhost/cleanup-dockerhosts

unit-test: build
	CONTIV_HOST_GOBIN=$(HOST_GOBIN) CONTIV_HOST_GOROOT=$(HOST_GOROOT) ./scripts/unittests -vagrant

system-test: build
	go test -v -run "sanity" github.com/contiv/netplugin/systemtests/singlehost 
	go test --timeout 20m -v -run "sanity" github.com/contiv/netplugin/systemtests/twohosts

regress-test: build
	go test -v -run "regress" github.com/contiv/netplugin/systemtests/singlehost 
	go test --timeout 60m -v -run "regress" github.com/contiv/netplugin/systemtests/twohosts
