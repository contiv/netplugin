
.PHONY: all all-CI build clean default system-test unit-test release tar

# find all verifiable packages.
# XXX: explore a better way that doesn't need multiple 'find'
PKGS := `find . -mindepth 1 -maxdepth 1 -type d -name '*' | grep -vE '/\..*$\|Godeps|examples|docs|scripts|mgmtfn|systemtests|bin'`
PKGS += `find . -mindepth 2 -maxdepth 2 -type d -name '*'| grep -vE '/\..*$\|Godeps|examples|docs|scripts|bin'`
TO_BUILD := ./netplugin/ ./netmaster/ ./netdcli/ ./mgmtfn/k8contivnet/ ./mgmtfn/dockcontivnet/ ./contivctl/contivctl/
HOST_GOBIN := `if [ -n "$$(go env GOBIN)" ]; then go env GOBIN; else dirname $$(which go); fi`
HOST_GOROOT := `go env GOROOT`
NAME := netplugin
# We are using date based versioning, so for consistent version during a build
# we evaluate and set the value of version once in a file and use it in 'tar'
# and 'release' targets.
VERSION_FILE := /tmp/$(NAME)-version
VERSION := `cat $(VERSION_FILE)`
TAR_EXT := tar.bz2
TAR_FILENAME := $(NAME)-$(VERSION).$(TAR_EXT)
TAR_LOC := .
TAR_FILE := $(TAR_LOC)/$(TAR_FILENAME)

all: build unit-test system-test system-test-dind centos-tests

# 'all-CI' target is used by the scripts/CI.sh that passes appropriate set of
# ENV variables (from the jenkins job) to run OS (centos, ubuntu etc) and
# sandbox specific(vagrant, docker-in-docker)
all-CI: build unit-test system-test

test: build unit-test system-test centos-tests

default: build

deps:
	./scripts/deps

checks:
	./scripts/checks "$(PKGS)"

host-build:
	sudo /bin/bash -c 'source /etc/profile.d/envvar.sh; make run-build'

run-build: deps checks clean
	godep go install -v $(TO_BUILD)
	cp ./scripts/python/contivctl.py $(GOPATH)/bin/contivctl

build:
	make start
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	make stop

clean: deps
	rm -rf Godeps/_workspace/pkg
	rm -rf $(GOPATH)/pkg
	godep go clean -i -v ./...

update:
	vagrant box update

# setting CONTIV_NODES=<number> while calling 'make demo' can be used to bring
# up a cluster of <number> nodes. By default <number> = 1
start: update
	CONTIV_NODE_OS=${CONTIV_NODE_OS} vagrant up

demo-centos:
	CONTIV_NODE_OS=centos make demo

stop:
	CONTIV_NODES=$${CONTIV_NODES:-2} vagrant destroy -f

demo: stop start

start-dockerdemo:
	scripts/dockerhost/start-dockerhosts

clean-dockerdemo:
	scripts/dockerhost/cleanup-dockerhosts

ssh:
	@vagrant ssh netplugin-node1 || echo 'Please run "make demo"'

unit-test: stop clean build
	./scripts/unittests -vagrant

unit-test-centos: stop
	CONTIV_NODE_OS=centos make clean build
	CONTIV_NODE_OS=centos ./scripts/unittests -vagrant

# setting CONTIV_SOE=1 while calling 'make system-test' will stop the test
# on first failure and leave setup in that state. This can be useful for debugging
# as part of development.
system-test: system-test-singlehost system-test-multihost

# the `make stop` here and below are necessary because build leaves around a VM (intentionally)
system-test-singlehost: stop clean checks
	make build stop
	godep go test -v --timeout 30m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/singlehost

system-test-multihost: stop clean checks build
	make build stop
	godep go test -v --timeout 80m -run "sanity" \
					   github.com/contiv/netplugin/systemtests/twohosts

system-test-centos: stop clean
	CONTIV_NODE_OS=centos make build stop system-test-singlehost system-test-multihost

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

tar: clean-tar build
	@echo "v0.0.0-`date -u +%m-%d-%Y.%H-%M-%S.UTC`" > $(VERSION_FILE)
	@tar -jcf $(TAR_FILE) -C $(GOPATH)/bin netplugin netdcli netmaster dockcontivnet k8contivnet

clean-tar:
	@rm -f $(TAR_LOC)/*.$(TAR_EXT)

# GITHUB_USER and GITHUB_TOKEN are needed be set to run github-release
release: tar
	@latest_tag=$$(git describe --tags `git rev-list --tags --max-count=1`); \
		comparison="$$latest_tag..HEAD"; \
		changelog=$$(git log $$comparison --oneline --no-merges --reverse); \
		if [ -z "$$changelog" ]; then echo "No new changes to release!"; exit 0; fi; \
		set -x; \
		( ( github-release -v release -p -r netplugin -t $(VERSION) -d "**Changelog**<br/>$$changelog" ) && \
		( github-release -v upload -r netplugin -t $(VERSION) -n $(TAR_FILENAME) -f $(TAR_FILE) || \
		github-release -v delete -r netplugin -t $(VERSION) ) ) || exit 1
	@make clean-tar
