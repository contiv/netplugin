
.PHONY: all build clean default system-test unit-test

# find all verifiable packages.
# XXX: explore a better way that doesn't need multiple 'find'
PKGS := `find . -mindepth 1 -maxdepth 1 -type d -name '*' | grep -vE '/\..*$\|Godeps|examples|docs|scripts|mgmtfn|systemtests|bin'`
PKGS += `find . -mindepth 2 -maxdepth 2 -type d -name '*'| grep -vE '/\..*$\|Godeps|examples|docs|scripts|bin'`
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

run-build: deps checks
	godep go install -v $(TO_BUILD)

build: start
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/golang/src/github.com/contiv/netplugin && make run-build"'

clean: deps
	rm -rf Godeps/_workspace/pkg
	godep go clean -i -v ./...

# setting CONTIV_NODES=<number> while calling 'make demo' can be used to bring
# up a cluster of <number> nodes. By default <number> = 1
start: 
	vagrant up

stop:
	vagrant destroy -f

demo: start build

clean-demo:
	vagrant destroy -f

start-dockerdemo:
	scripts/dockerhost/start-dockerhosts

clean-dockerdemo:
	scripts/dockerhost/cleanup-dockerhosts

ssh:
	@vagrant ssh netplugin-node1 || echo 'Please run "make demo"'

unit-test: build clean
	./scripts/unittests -vagrant

unit-test-centos: build clean
	CONTIV_NODE_OS=centos ./scripts/unittests -vagrant

# setting CONTIV_SOE=1 while calling 'make system-test' will stop the test
# on first failure and leave setup in that state. This can be useful for debugging
# as part of development.
system-test: build clean
	godep go test --timeout 30m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/singlehost 
	godep go test --timeout 80m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/twohosts

system-test-centos: build clean
	CONTIV_NODE_OS=centos godep go test --timeout 30m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/singlehost
	CONTIV_NODE_OS=centos godep go test --timeout 90m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/twohosts

centos-tests: unit-test-centos system-test-centos

# setting CONTIV_SOE=1 while calling 'make regress-test' will stop the test
# on first failure and leave setup in that state. This can be useful for debugging
# as part of development.
regress-test: build
	godep go test -run "regress" \
					   github.com/contiv/netplugin/systemtests/singlehost
	godep go test --timeout 60m -run "regress" \
					   github.com/contiv/netplugin/systemtests/twohosts

# Setting CONTIV_TESTBED=DIND uses docker in docker as the testbed instead of vagrant VMs.
system-test-dind:
	CONTIV_TESTBED=DIND make system-test

regress-test-dind:
	CONTIV_TESTBED=DIND make regress-test
