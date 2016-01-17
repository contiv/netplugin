
.PHONY: all all-CI build clean default unit-test release tar

# find all verifiable packages.
# XXX: explore a better way that doesn't need multiple 'find'
PKGS := `find . -mindepth 1 -maxdepth 1 -type d -name '*' | grep -vE '/\..*$\|Godeps|examples|docs|scripts|mgmtfn|bin'`
PKGS += `find . -mindepth 2 -maxdepth 2 -type d -name '*'| grep -vE '/\..*$\|Godeps|examples|docs|scripts|bin'`
TO_BUILD := ./netplugin/ ./netmaster/ ./netctl/netctl/
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

all: build unit-test sanity-test centos-tests

# 'all-CI' target is used by the scripts/CI.sh that passes appropriate set of
# ENV variables (from the jenkins job) to run OS (centos, ubuntu etc) and
# sandbox specific(vagrant, docker-in-docker)
all-CI: build unit-test sanity-test

test: build unit-test sanity-test centos-tests

default: build

deps:
	./scripts/deps

checks:
	./scripts/checks "$(PKGS)"

run-build: deps checks clean
	cd Godeps/_workspace/src/github.com/contiv/ && chmod -R 771 contivmodel/ && cd contivmodel/ && ./generate.sh && \
	cd /opt/gopath/src/github.com/contiv/netplugin && \
	godep go install -v $(TO_BUILD) 

build:
	make start
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	make stop

clean: deps
	rm -rf Godeps/_workspace/pkg
	rm -rf $(GOPATH)/pkg/*
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

demo:
	vagrant up
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	vagrant ssh netplugin-node1 -c 'nohup bash -lc "sudo /opt/gopath/bin/netplugin -docker-plugin 2>&1> /tmp/netplugin.log &"'
	vagrant ssh netplugin-node2 -c 'nohup bash -lc "sudo /opt/gopath/bin/netplugin -docker-plugin 2>&1> /tmp/netplugin.log &"'
	sleep 10
	vagrant ssh netplugin-node1 -c 'nohup bash -lc "/opt/gopath/bin/netmaster 2>&1> /tmp/netmaster.log &"'
	sleep 10

ssh:
	@vagrant ssh netplugin-node1 || echo 'Please run "make demo"'

unit-test: stop clean build
	./scripts/unittests -vagrant

centos-tests:
	CONTIV_NODE_OS=centos make clean build unit-test sanity-test stop

sanity-test: start
	vagrant ssh netplugin-node1 -c 'bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./sanity.py -nodes 192.168.2.10,192.168.2.11"'

host-build:
	@echo "dev: making binaries..."
	sudo /bin/bash -c 'source /etc/profile.d/envvar.sh; make run-build'

host-unit-test:
	@echo dev: running unit tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin && sudo -E PATH=$(PATH) scripts/unittests

host-sanity-test:
	@echo dev: running sanity tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./sanity.py -nodes ${CLUSTER_NODE_IPS}

host-cleanup:
	@echo dev: cleaning up, restarting services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./cleanup.py -nodes ${CLUSTER_NODE_IPS}

tar: clean-tar build
	@echo "v0.0.0-`date -u +%m-%d-%Y.%H-%M-%S.UTC`" > $(VERSION_FILE)
	@tar -jcf $(TAR_FILE) -C $(GOPATH)/src/github.com/contiv/netplugin/bin netplugin netmaster netctl

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
