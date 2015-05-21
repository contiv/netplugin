
.PHONY: all build clean default system-test unit-test

TO_BUILD := ./netplugin/ ./netdcli/ ./mgmtfn/k8contivnet/ ./mgmtfn/pslibnet/
HOST_GOBIN := `which go | xargs dirname`
HOST_GOROOT := `go env GOROOT`

all: build unit-test system-test system-test-dind

default: build

deps:
	./scripts/deps

build: deps
	./scripts/checks "$(TO_BUILD)"
	godep go install -v $(TO_BUILD)

clean: deps
	godep go clean -i -v ./...

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
	CONTIV_HOST_GOPATH=$(GOPATH) CONTIV_HOST_GOBIN=$(HOST_GOBIN) \
					   CONTIV_HOST_GOROOT=$(HOST_GOROOT) ./scripts/unittests -vagrant

# setting CONTIV_SOE=1 while calling 'make system-test' will stop the test
# on first failure and leave setup in that state. This can be useful for debugging
# as part of development.
system-test: build
	CONTIV_HOST_GOPATH=$(GOPATH) godep go test -v -run "sanity" \
					   github.com/contiv/netplugin/systemtests/singlehost 
	CONTIV_HOST_GOPATH=$(GOPATH) godep go test --timeout 20m -v -run "sanity" \
					   github.com/contiv/netplugin/systemtests/twohosts

# setting CONTIV_SOE=1 while calling 'make regress-test' will stop the test
# on first failure and leave setup in that state. This can be useful for debugging
# as part of development.
regress-test: build
	CONTIV_HOST_GOPATH=$(GOPATH) godep go test -v -run "regress" \
					   github.com/contiv/netplugin/systemtests/singlehost 
	CONTIV_HOST_GOPATH=$(GOPATH) godep go test --timeout 60m -v -run "regress" \
					   github.com/contiv/netplugin/systemtests/twohosts

# Setting CONTIV_TESTBED=DIND uses docker in docker as the testbed instead of vagrant VMs. 
system-test-dind:
	CONTIV_TESTBED=DIND make system-test

regress-test-dind:
	CONTIV_TESTBED=DIND make regress-test
