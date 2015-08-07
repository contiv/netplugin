
.PHONY: all build clean default system-test unit-test

# find all verifiable packages.
# XXX: explore a better way that doesn't need multiple 'find'
PKGS := `find . -mindepth 1 -maxdepth 1 -type d -name '*' | grep -vE '/\..*$\|Godeps|examples|docs|scripts|mgmtfn|systemtests'`
PKGS += `find . -mindepth 2 -maxdepth 2 -type d -name '*'| grep -vE '/\..*$\|Godeps|examples|docs|scripts'`
TO_BUILD := ./netplugin/ ./netmaster/ ./netdcli/ ./mgmtfn/k8contivnet/ ./mgmtfn/pslibnet/ ./mgmtfn/dockcontivnet/
HOST_GOBIN := `if [ -n "$$(go env GOBIN)" ]; then go env GOBIN; else dirname $$(which go); fi`
HOST_GOROOT := `go env GOROOT`

all: build unit-test system-test system-test-dind centos-tests

# 'all-CI' target is used by the scripts/CI.sh that passes appropriate set of
# ENV variables (from the jenkins job) to run OS (centos, ubuntu etc) and
# sandbox specific(vagrant, docker-in-docker)
all-CI: build unit-test system-test

default: build

deps:
	./scripts/deps

checks:
	./scripts/checks "$(PKGS)"

build: deps checks
	godep go install -v $(TO_BUILD)

clean: deps
	rm -rf Godeps/_workspace/pkg
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

ssh:
	@vagrant ssh netplugin-node1 || echo 'Please run "make demo"'

unit-test: build
	CONTIV_HOST_GOPATH=$(GOPATH) CONTIV_HOST_GOBIN=$(HOST_GOBIN) \
					   CONTIV_HOST_GOROOT=$(HOST_GOROOT) ./scripts/unittests -vagrant

unit-test-centos: build
	CONTIV_NODE_OS=centos CONTIV_HOST_GOPATH=$(GOPATH) CONTIV_HOST_GOBIN=$(HOST_GOBIN) \
					   CONTIV_HOST_GOROOT=$(HOST_GOROOT) ./scripts/unittests -vagrant

# setting CONTIV_SOE=1 while calling 'make system-test' will stop the test
# on first failure and leave setup in that state. This can be useful for debugging
# as part of development.
system-test: build
	CONTIV_HOST_GOROOT=$(HOST_GOROOT) CONTIV_HOST_GOBIN=$(HOST_GOBIN) CONTIV_HOST_GOPATH=$(GOPATH) godep go test --timeout 30m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/singlehost 
	CONTIV_HOST_GOROOT=$(HOST_GOROOT) CONTIV_HOST_GOBIN=$(HOST_GOBIN) CONTIV_HOST_GOPATH=$(GOPATH) godep go test --timeout 80m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/twohosts

system-test-centos: build
	CONTIV_NODE_OS=centos CONTIV_HOST_GOPATH=$(GOPATH) godep go test --timeout 30m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/singlehost
	CONTIV_NODE_OS=centos CONTIV_HOST_GOPATH=$(GOPATH) godep go test --timeout 90m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/twohosts

centos-tests: unit-test-centos system-test-centos

# setting CONTIV_SOE=1 while calling 'make regress-test' will stop the test
# on first failure and leave setup in that state. This can be useful for debugging
# as part of development.
regress-test: build
	CONTIV_HOST_GOPATH=$(GOPATH) godep go test -run "regress" \
					   github.com/contiv/netplugin/systemtests/singlehost
	CONTIV_HOST_GOPATH=$(GOPATH) godep go test --timeout 60m -run "regress" \
					   github.com/contiv/netplugin/systemtests/twohosts

# Setting CONTIV_TESTBED=DIND uses docker in docker as the testbed instead of vagrant VMs.
system-test-dind:
	CONTIV_TESTBED=DIND make system-test

regress-test-dind:
	CONTIV_TESTBED=DIND make regress-test
