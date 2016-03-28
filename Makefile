
.PHONY: all all-CI build clean default unit-test release tar

# find all verifiable packages.
# XXX: explore a better way that doesn't need multiple 'find'
PKGS := `find . -mindepth 1 -maxdepth 1 -type d -name '*' | grep -vE '/\..*$\|Godeps|examples|docs|scripts|mgmtfn|bin|contrib|demo'`
PKGS += `find . -mindepth 2 -maxdepth 2 -type d -name '*'| grep -vE '/\..*$\|Godeps|examples|docs|scripts|bin|contrib|demo'`
TO_BUILD := ./netplugin/ ./netmaster/ ./netctl/netctl/ ./mgmtfn/k8splugin/contivk8s/
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

all: build unit-test system-test centos-tests

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

run-build: deps checks clean
	cd ${GOPATH}/src/github.com/contiv/netplugin && version/generate_version ${USE_RELEASE} && \
	cd $(GOPATH)/src/github.com/contiv/netplugin && \
	godep go install -v $(TO_BUILD) && \
	sudo cp contrib/completion/bash/netctl /etc/bash_completion.d/netctl

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

#kubernetes demo targets
k8s-cluster:
	cd demo/k8s/ && ./setup_cluster.sh
k8s-demo:
	cd demo/k8s/ && ./copy_demo.sh
k8s-demo-start:
	cd demo/k8s/ && ./restart_cluster.sh && vagrant ssh k8master
k8s-destroy:
	cd demo/k8s/ && vagrant destroy -f

# Mesos demo targets
mesos-docker-demo:
	cd demo/mesos-docker && vagrant up
	cd demo/mesos-docker && vagrant ssh node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	cd demo/mesos-docker && vagrant ssh node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && ./scripts/python/startPlugin.py -nodes 192.168.33.10,192.168.33.11"'

mesos-docker-destroy:
	cd demo/mesos-docker && vagrant destroy -f


demo-centos:
	CONTIV_NODE_OS=centos make demo

stop:
	CONTIV_NODES=$${CONTIV_NODES:-2} vagrant destroy -f

demo:
	vagrant up
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-restart"'

ssh:
	@vagrant ssh netplugin-node1 -c 'bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin/ && bash"' || echo 'Please run "make demo"'

unit-test: stop clean build
	./scripts/unittests -vagrant

centos-tests:
	CONTIV_NODE_OS=centos make clean build unit-test system-test stop

system-test: start
	godep go test -v -timeout 120m ./systemtests -check.v

host-build:
	@echo "dev: making binaries..."
	sudo /bin/bash -c 'source /etc/profile.d/envvar.sh; make run-build'

host-unit-test:
	@echo dev: running unit tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin && sudo -E PATH=$(PATH) scripts/unittests

host-unit-test-coverage:
	@echo dev: running unit tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin && sudo -E PATH=$(PATH) scripts/unittests --coverage-basic

host-unit-test-coverage-detail:
	@echo dev: running unit tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin && sudo -E PATH=$(PATH) scripts/unittests --coverage-detail

host-sanity-test:
	@echo dev: running sanity tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./sanity.py -nodes ${CLUSTER_NODE_IPS}

host-short-sanity-test:
	@echo dev: running sanity tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./sanity.py -short true -nodes ${CLUSTER_NODE_IPS}

host-cleanup:
	@echo dev: cleaning up services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./cleanup.py -nodes ${CLUSTER_NODE_IPS}

host-restart:
	@echo dev: restarting services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./startPlugin.py -nodes ${CLUSTER_NODE_IPS}

only-tar:

tar: clean-tar build
	@cat ${GOPATH}/src/github.com/contiv/netplugin/version/version_gen.go | grep versionStr | cut -f 4 -d " " | tr -d \" > $(VERSION_FILE)
	@tar -jcf $(TAR_FILE) -C $(GOPATH)/src/github.com/contiv/netplugin/bin netplugin netmaster netctl contivk8s -C $(GOPATH)/src/github.com/contiv/netplugin contrib/completion/bash/netctl

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
