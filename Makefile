
.PHONY: all all-CI build clean default unit-test release tar checks go-version gofmt-src golint-src govet-src

SHELL := /bin/bash
EXCLUDE_DIRS := bin docs Godeps scripts test vagrant vendor
PKG_DIRS := $(filter-out $(EXCLUDE_DIRS),$(subst /,,$(sort $(dir $(wildcard */)))))
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
GO_MIN_VERSION := 1.5.1
GO_MAX_VERSION := 1.6.2
GO_VERSION := $(shell go version | cut -d' ' -f3 | sed 's/go//')
GOLINT_CMD := golint -set_exit_status
GOFMT_CMD := gofmt -l
GOVET_CMD := go tool vet

all: build unit-test system-test ubuntu-tests

# 'all-CI' target is used by the scripts/CI.sh that passes appropriate set of
# ENV variables (from the jenkins job) to run OS (centos, ubuntu etc) and
# sandbox specific(vagrant, docker-in-docker)
all-CI: stop clean start
	make ssh-build
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-unit-test"'
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-integ-test"'
	make system-test

test: build unit-test system-test ubuntu-tests

default: build

deps:
	./scripts/deps

gofmt-src: $(PKG_DIRS)
	$(info +++ gofmt $(PKG_DIRS))
	@for dir in $?; do $(GOFMT_CMD) $${dir} | grep "go"; [[ $$? -ne 0 ]] || exit 1; done

golint-src: $(PKG_DIRS)
	$(info +++ golint $(PKG_DIRS))
	@for dir in $?; do $(GOLINT_CMD) $${dir}/... || exit 1;done

govet-src: $(PKG_DIRS)
	$(info +++ govet $(PKG_DIRS))
	@for dir in $?; do $(GOVET_CMD) $${dir} || exit 1;done

go-version:
	$(info +++ check go version)
ifneq ($(GO_VERSION), $(lastword $(sort $(GO_VERSION) $(GO_MIN_VERSION))))
	$(error go version check failed, expected >= $(GO_MIN_VERSION), found $(GO_VERSION))
endif
ifneq ($(GO_VERSION), $(firstword $(sort $(GO_VERSION) $(GO_MAX_VERSION))))
	$(error go version check failed, expected <= $(GO_MAX_VERSION), found $(GO_VERSION))
endif

checks: go-version gofmt-src golint-src govet-src

# We cannot perform sudo inside a golang, the only reason to split the rules
# here
ifdef NET_CONTAINER_BUILD
run-build: deps checks clean
	cd ${GOPATH}/src/github.com/contiv/netplugin && version/generate_version ${USE_RELEASE} && \
	cd $(GOPATH)/src/github.com/contiv/netplugin && \
	GOGC=1500 go install -v $(TO_BUILD) && \
	cp scripts/contrib/completion/bash/netctl /etc/bash_completion.d/netctl
else
run-build: deps checks clean
	cd ${GOPATH}/src/github.com/contiv/netplugin && version/generate_version ${USE_RELEASE} && \
	cd $(GOPATH)/src/github.com/contiv/netplugin && \
	GOGC=1500 go install -v $(TO_BUILD) && \
	sudo cp scripts/contrib/completion/bash/netctl /etc/bash_completion.d/netctl
endif

build:
	make start
	make ssh-build
	make stop

clean: deps
	rm -rf $(GOPATH)/pkg/*
	go clean -i -v ./...

update:
	vagrant box update


# setting CONTIV_NODES=<number> while calling 'make demo' can be used to bring
# up a cluster of <number> nodes. By default <number> = 1
ifdef NET_CONTAINER_BUILD
start:
else
start: 
	CONTIV_NODE_OS=${CONTIV_NODE_OS} vagrant up
endif

#kubernetes demo targets
k8s-cluster:
	cd vagrant/k8s/ && ./setup_cluster.sh
k8s-demo:
	cd vagrant/k8s/ && ./copy_demo.sh
k8s-demo-start:
	cd vagrant/k8s/ && ./restart_cluster.sh && vagrant ssh k8master
k8s-destroy:
	cd vagrant/k8s/ && vagrant destroy -f
k8s-sanity-cluster:
	cd vagrant/k8s/ && ./setup_cluster.sh
k8s-test:
	CONTIV_K8=1 make k8s-sanity-cluster
	#make ssh-build 
	cd vagrant/k8s/ && CONTIV_K8=1 vagrant ssh k8master -c 'sudo -i bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	CONTIV_K8=1 cd vagrant/k8s/ && ./start_sanity_service.sh
	CONTIV_K8=1 CONTIV_NODES=3 go test -v -timeout 540m ./test/systemtests -check.v -check.f "00SSH|Basic|Network|Policy|TestTrigger|ACIM|HostBridge" 
	cd vagrant/k8s && vagrant destroy -f 
# Mesos demo targets
mesos-docker-demo:
	cd vagrant/mesos-docker && vagrant up
	cd vagrant/mesos-docker && vagrant ssh node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	cd vagrant/mesos-docker && vagrant ssh node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && ./scripts/python/startPlugin.py -nodes 192.168.33.10,192.168.33.11"'

mesos-docker-destroy:
	cd vagrant/mesos-docker && vagrant destroy -f

nomad-docker:
	cd vagrant/nomad-docker && vagrant up
	VAGRANT_CWD=./vagrant/nomad-docker/ vagrant ssh netplugin-node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-restart"'
demo-ubuntu:
	CONTIV_NODE_OS=ubuntu make demo

ifdef NET_CONTAINER_BUILD
stop:
else
stop:
	CONTIV_NODES=$${CONTIV_NODES:-3} vagrant destroy -f
endif

demo:
	make ssh-build
	vagrant ssh netplugin-node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-restart && make host-swarm-restart"'

ssh:
	@vagrant ssh netplugin-node1 -c 'bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin/ && bash"' || echo 'Please run "make demo"'

ifdef NET_CONTAINER_BUILD
ssh-build:
	cd /go/src/github.com/contiv/netplugin && make run-build
else
ssh-build: start
		vagrant ssh netplugin-node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
endif

unit-test: stop clean build
	./scripts/unittests -vagrant

ubuntu-tests:
	CONTIV_NODE_OS=ubuntu make clean build unit-test system-test stop

system-test:start
	go test -v -timeout 480m ./test/systemtests -check.vv -check.f "00SSH|Basic|Network|Policy|TestTrigger|ACIM"

l3-test:
	CONTIV_L3=2 CONTIV_NODES=3 make stop
	CONTIV_L3=2 CONTIV_NODES=3 make start
	CONTIV_L3=2 CONTIV_NODES=3 make ssh-build
	CONTIV_L3=2 CONTIV_NODES=3 go test -v -timeout 540m ./test/systemtests -check.v  
	CONTIV_L3=2 CONTIV_NODES=3 make stop
l3-demo:
	CONTIV_L3=1 CONTIV_NODES=3 vagrant up
	make ssh-build
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin && make host-restart"'

host-build:
	@echo "dev: making binaries..."
	/bin/bash -c 'source /etc/profile.d/envvar.sh; make run-build'

host-unit-test:
	@echo dev: running unit tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin && sudo -E PATH=$(PATH) scripts/unittests

host-unit-test-coverage:
	@echo dev: running unit tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin && sudo -E PATH=$(PATH) scripts/unittests --coverage-basic

host-unit-test-coverage-detail:
	@echo dev: running unit tests...
	cd $(GOPATH)/src/github.com/contiv/netplugin && sudo -E PATH=$(PATH) scripts/unittests --coverage-detail

host-integ-test: host-cleanup
	@echo dev: running integration tests...
	sudo -E /usr/local/go/bin/go test -v ./test/integration/ -check.vv -encap vlan -fwd-mode bridge
	sudo -E /usr/local/go/bin/go test -v ./test/integration/ -check.vv -encap vxlan -fwd-mode bridge
	sudo -E /usr/local/go/bin/go test -v ./test/integration/ -check.vv -encap vxlan -fwd-mode routing

host-cleanup:
	@echo dev: cleaning up services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./cleanup.py -nodes ${CLUSTER_NODE_IPS}

host-swarm-restart:
	@echo dev: restarting swarm ...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./startSwarm.py -nodes ${CLUSTER_NODE_IPS}

host-restart:
	@echo dev: restarting services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./startPlugin.py -nodes ${CLUSTER_NODE_IPS}

only-tar:

tar: clean-tar build
	@cat ${GOPATH}/src/github.com/contiv/netplugin/version/version_gen.go | grep versionStr | cut -f 4 -d " " | tr -d \" > $(VERSION_FILE)
	@tar -jcf $(TAR_FILE) -C $(GOPATH)/src/github.com/contiv/netplugin/bin netplugin netmaster netctl contivk8s -C $(GOPATH)/src/github.com/contiv/netplugin/scripts contrib/completion/bash/netctl

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
