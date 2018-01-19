# BUILD_VERSION will affect archive filenames as well as -version
# default version will be based on $(git describe --tags --always)


.PHONY: all all-CI build clean default unit-test release tar checks go-version gofmt-src \
	golint-src govet-src run-build compile-with-docker

DEFAULT_DOCKER_VERSION := 1.12.6
V2PLUGIN_DOCKER_VERSION := 1.13.1
CONTIV_K8S_VERSION ?= stable
SHELL := /bin/bash
# TODO: contivmodel should be removed once its code passes golint and misspell
EXCLUDE_DIRS := bin docs Godeps scripts vagrant vendor install contivmodel
PKG_DIRS := $(filter-out $(EXCLUDE_DIRS),$(subst /,,$(sort $(dir $(wildcard */)))))
TO_BUILD := ./netplugin/ ./netmaster/ ./netctl/netctl/ ./mgmtfn/k8splugin/contivk8s/ ./mgmtfn/mesosplugin/netcontiv/
HOST_GOBIN := `if [ -n "$$(go env GOBIN)" ]; then go env GOBIN; else dirname $$(which go); fi`
HOST_GOROOT := `go env GOROOT`
NAME := netplugin
VERSION := $(shell scripts/getGitVersion.sh)
TAR := $(shell command -v gtar || command -v tar || echo "Could not find tar")
TAR_EXT := tar.bz2
export NETPLUGIN_CONTAINER_TAG := $(shell ./scripts/getGitVersion.sh)
TAR_FILENAME := $(NAME)-$(VERSION).$(TAR_EXT)
TAR_LOC := .
export TAR_FILE := $(TAR_LOC)/$(TAR_FILENAME)
export V2PLUGIN_TAR_FILENAME := v2plugin-$(VERSION).tar.gz
GO_MIN_VERSION := 1.7
GO_MAX_VERSION := 1.8
GO_VERSION := $(shell go version | cut -d' ' -f3 | sed 's/go//')
CI_HOST_TARGETS ?= "host-unit-test host-integ-test host-build-docker-image tar host-pluginfs-create clean-tar"
SYSTEM_TESTS_TO_RUN ?= "00SSH|Basic|Network|Policy|TestTrigger|ACIM|Netprofile"
K8S_SYSTEM_TESTS_TO_RUN ?= "00SSH|Basic|Network|Policy"
ACI_GW_IMAGE ?= "contiv/aci-gw:04-12-2017.2.2_1n"
export CONTIV_V2PLUGIN_NAME ?= contiv/v2plugin:0.1

all: build unit-test system-test ubuntu-tests

# 'all-CI' target is used by the scripts/CI.sh that passes appropriate set of
# ENV variables (from the jenkins job) to run OS (centos, ubuntu etc) and
# sandbox specific(vagrant, docker-in-docker)
all-CI: stop clean start
	make ssh-build
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh \
		&& cd /opt/gopath/src/github.com/contiv/netplugin \
		&& make ${CI_HOST_TARGETS}"'
ifdef SKIP_SYSTEM_TEST
	@echo "Skipping system tests"
else
	make system-test
endif

test: build unit-test system-test ubuntu-tests

default: build

deps:
	./scripts/deps

godep-save:
	rm -rf vendor Godeps
	godep save ./...

godep-restore:
	godep restore ./...

gofmt-src:
	$(info +++ gofmt $(PKG_DIRS))
	@[[ -z "$$(gofmt -s -d $(PKG_DIRS) | tee /dev/stderr)" ]] || exit 1

# go lint does not automatically recurse
golint-src:
	$(info +++ golint $(PKG_DIRS))
	@for dir in $(PKG_DIRS); do golint -set_exit_status $${dir}/...; done

govet-src:
	$(info +++ govet $(PKG_DIRS))
	@go tool vet $(PKG_DIRS)

misspell-src:
	$(info +++ check spelling $(PKG_DIRS))
	@misspell -locale US -error $(PKG_DIRS)

go-version:
	$(info +++ check go version)
ifneq ($(GO_VERSION), $(lastword $(sort $(GO_VERSION) $(GO_MIN_VERSION))))
	$(error go version check failed, expected >= $(GO_MIN_VERSION), found $(GO_VERSION))
endif
ifneq ($(GO_VERSION), $(firstword $(sort $(GO_VERSION) $(GO_MAX_VERSION))))
	$(error go version check failed, expected <= $(GO_MAX_VERSION), found $(GO_VERSION))
endif

checks: go-version govet-src golint-src gofmt-src misspell-src

# When multi-stage builds are available in VM, source can be copied into
# container FROM the netplugin-build container to simplify this target
checks-with-docker:
	scripts/code_checks_in_docker.sh $(PKG_DIRS)

# install binaries into GOPATH and update file netplugin-version
compile:
	cd $(GOPATH)/src/github.com/contiv/netplugin && \
	NIGHTLY_RELEASE=${NIGHTLY_RELEASE} TO_BUILD="${TO_BUILD}" \
	BUILD_VERSION=$(VERSION) scripts/build.sh

# fully prepares code for pushing to branch, includes building binaries
run-build: deps checks clean compile archive

compile-with-docker:
	docker build \
		-f Dockerfile-compile \
		--build-arg NIGHTLY_RELEASE=$(NIGHTLY_RELEASE) \
		--build-arg BUILD_VERSION=$(VERSION) \
		-t netplugin-build:$(NETPLUGIN_CONTAINER_TAG) .

build-docker-image: start
	vagrant ssh netplugin-node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-build-docker-image"'

install-shell-completion:
	sudo cp scripts/contrib/completion/bash/netctl /etc/bash_completion.d/netctl

build: start ssh-build stop

clean: deps
	rm -rf $(GOPATH)/pkg/*/github.com/contiv/netplugin/
	go clean -i -v ./...

update:
	vagrant box update


# setting CONTIV_NODES=<number> while calling 'make demo' can be used to bring
# up a cluster of <number> nodes. By default <number> = 1
start:
	CONTIV_DOCKER_VERSION="$${CONTIV_DOCKER_VERSION:-$(DEFAULT_DOCKER_VERSION)}" CONTIV_NODE_OS=${CONTIV_NODE_OS} vagrant up

# ===================================================================
# kubernetes cluster bringup/cleanup targets
k8s-cluster:
	vagrant plugin install vagrant-cachier || echo "failed install vagrant-cachier"
	cd vagrant/k8s/ && CONTIV_K8S_VERSION=$(CONTIV_K8S_VERSION) vagrant up

k8s-l3-cluster:
	CONTIV_L3=1 make k8s-cluster

k8s-destroy:
	cd vagrant/k8s/ && vagrant destroy -f

k8s-l3-destroy:
	cd vagrant/k8s/ && CONTIV_L3=1 vagrant destroy -f

# ===================================================================
# kubernetes dev
k8s-dev: checks-with-docker compile-with-docker binaries-from-container
	CONTIV_TEST="dev" make k8s-cluster

# kubernetes test targets
k8s-test: checks-with-docker compile-with-docker binaries-from-container
	CONTIV_TEST="sys" make k8s-cluster
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./createcfg.py -scheduler 'k8s' -binpath contiv/bin -install_mode 'kubeadm'
	CONTIV_K8S_USE_KUBEADM=1 CONTIV_NODES=3 go test -v -timeout 540m ./test/systemtests -check.v -check.abort -check.f $(K8S_SYSTEM_TESTS_TO_RUN)
	cd vagrant/k8s && vagrant destroy -f

k8s-l3-test: k8s-l3-cluster
	cd vagrant/k8s/ && vagrant ssh k8master -c 'bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"'
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./createcfg.py -scheduler 'k8s' -binpath contiv/bin -install_mode 'kubeadm' -contiv_l3=1
	CONTIV_K8S_USE_KUBEADM=1 CONTIV_NODES=3 go test -v -timeout 540m ./test/systemtests -check.v -check.abort -check.f $(K8S_SYSTEM_TESTS_TO_RUN)
	cd vagrant/k8s && CONTIV_L3=1 vagrant destroy -f
# ===================================================================

# Mesos demo targets
mesos-docker-demo:
	cd vagrant/mesos-docker && \
	vagrant up && \
	vagrant ssh node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build"' && \
	vagrant ssh node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && ./scripts/python/startPlugin.py -nodes 192.168.33.10,192.168.33.11"'

mesos-docker-destroy:
	cd vagrant/mesos-docker && vagrant destroy -f

nomad-docker:
	cd vagrant/nomad-docker && vagrant up
	VAGRANT_CWD=./vagrant/nomad-docker/ vagrant ssh netplugin-node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-restart"'

mesos-cni-demo:
	$(MAKE) -C vagrant/mesos-cni $@

mesos-cni-destroy:
	$(MAKE) -C vagrant/mesos-cni $@

demo-ubuntu:
	CONTIV_NODE_OS=ubuntu make demo

stop:
	CONTIV_NODES=$${CONTIV_NODES:-3} vagrant destroy -f

demo: ssh-build
	vagrant ssh netplugin-node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-restart host-swarm-restart"'

ssh:
	@vagrant ssh netplugin-node1 -c 'bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin/ && bash"' || echo 'Please run "make demo"'

ssh-build: start
	vagrant ssh netplugin-node1 -c 'bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make run-build install-shell-completion"'

unit-test: stop clean
	./scripts/unittests -vagrant

integ-test: stop clean start ssh-build
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "source /etc/profile.d/envvar.sh && cd /opt/gopath/src/github.com/contiv/netplugin && make host-integ-test"'

ubuntu-tests:
	CONTIV_NODE_OS=ubuntu make clean build unit-test system-test stop

system-test:start
	@echo "system-test: running the following system tests:" $(SYSTEM_TESTS_TO_RUN)
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./createcfg.py
	go test -v -timeout 480m ./test/systemtests -check.v -check.abort -check.f $(SYSTEM_TESTS_TO_RUN)

l3-test:
	CONTIV_L3=2 CONTIV_NODES=3 make stop start ssh-build
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./createcfg.py -contiv_l3 2
	CONTIV_L3=2 CONTIV_NODES=3 go test -v -timeout 900m ./test/systemtests -check.v -check.abort
	CONTIV_L3=2 CONTIV_NODES=3 make stop

#l3-demo setup for docker/swarm
l3-demo: demo
	vagrant ssh netplugin-node1 -c 'netctl global set --fwd-mode routing'

l3bgp-demo:
	CONTIV_L3=1 CONTIV_NODES=3 vagrant up
	make ssh-build
	vagrant ssh netplugin-node1 -c 'sudo -i bash -lc "cd /opt/gopath/src/github.com/contiv/netplugin && make host-restart"'
	vagrant ssh netplugin-node1 -c 'sh /opt/gopath/src/github.com/contiv/netplugin/scripts/l3bgp_demo.sh'

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

host-integ-test: host-cleanup start-aci-gw
	@echo dev: running integration tests...
	sudo -E /usr/local/go/bin/go test -v -timeout 20m ./test/integration/ -check.v -encap vlan -fwd-mode bridge
	sudo -E /usr/local/go/bin/go test -v -timeout 20m ./test/integration/ -check.v -encap vxlan -fwd-mode bridge
	sudo -E /usr/local/go/bin/go test -v -timeout 20m ./test/integration/ -check.v -encap vxlan -fwd-mode routing
	sudo -E /usr/local/go/bin/go test -v -timeout 20m ./test/integration/ -check.v -check.f "AppProfile" -encap vlan -fwd-mode bridge --fabric-mode aci

start-aci-gw:
	@echo dev: starting aci gw...
	docker pull $(ACI_GW_IMAGE)
	docker run --net=host -itd -e "APIC_URL=SANITY" -e "APIC_USERNAME=IGNORE" -e "APIC_PASSWORD=IGNORE" --name=contiv-aci-gw $(ACI_GW_IMAGE)

host-build-docker-image: compile-with-docker binaries-from-container
	@./scripts/netContain/build_image.sh

host-cleanup:
	@echo dev: cleaning up services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./cleanup.py -nodes ${CLUSTER_NODE_IPS}

host-swarm-restart:
	@echo dev: restarting swarm ...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./startSwarm.py -nodes ${CLUSTER_NODE_IPS} -swarm ${CONTIV_DOCKER_SWARM}

host-restart:
	@echo dev: restarting services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python && PYTHONIOENCODING=utf-8 ./startPlugin.py -nodes ${CLUSTER_NODE_IPS}

# create the rootfs for v2plugin. this is required for docker plugin create command
host-pluginfs-create:
	./scripts/v2plugin_rootfs.sh

# remove the v2plugin from docker
host-plugin-remove:
	@echo dev: removing docker v2plugin ...
	docker plugin disable ${CONTIV_V2PLUGIN_NAME}
	docker plugin rm -f ${CONTIV_V2PLUGIN_NAME}

# add the v2plugin to docker with the current rootfs
host-plugin-create:
	@echo Creating docker v2plugin
	docker plugin create ${CONTIV_V2PLUGIN_NAME} install/v2plugin
	docker plugin enable ${CONTIV_V2PLUGIN_NAME}

# Shortcut for an existing v2plugin cluster to update the netplugin
# binaries.
# Recommended process after updating netplugin source:
#     make compile archive host-plugin-update
# Note: only updates a single host
# Note: only applies to v2plugin (which implies docker 1.13+)
host-plugin-update: host-plugin-remove unarchive host-plugin-create
# same behavior as host-plugin-update but runs locally with docker 1.13+
plugin-update: tar
	$(call make-on-node1, host-plugin-update)

# cleanup all containers, recreate and start the v2plugin on all hosts
# uses the latest compiled binaries
host-plugin-restart: unarchive
	@echo dev: restarting services...
	cd $(GOPATH)/src/github.com/contiv/netplugin/scripts/python \
		&& PYTHONIOENCODING=utf-8 ./startPlugin.py -nodes ${CLUSTER_NODE_IPS} \
			-plugintype "v2plugin"

# unpack v2plugin archive created by host-pluginfs-create
# Note: do not unpack locally to share with VM, unpack on the target machine
host-pluginfs-unpack:
	# clear out old plugin completely
	sudo rm -rf install/v2plugin/rootfs
	mkdir -p install/v2plugin/rootfs
	sudo tar -xf install/v2plugin/${V2PLUGIN_TAR_FILENAME} \
		-C install/v2plugin/rootfs/ \
		--exclude=usr/share/terminfo --exclude=dev/null \
		--exclude=etc/terminfo/v/vt220

# Runs make targets on the first netplugin vagrant node
# this is used as a macro like $(call make-on-node1, compile checks)
make-on-node1 = vagrant ssh netplugin-node1 -c '\
	bash -lc "source /etc/profile.d/envvar.sh \
	&& cd /opt/gopath/src/github.com/contiv/netplugin && make $(1)"'

# Calls macro make-on-node1 but can be used as a dependecy by setting
# the variable "node1-make-targets"
make-on-node1-dep:
	$(call make-on-node1, $(node1-make-targets))

# assumes the v2plugin archive is available, installs the v2plugin and resets
# everything on the vm to clean state
v2plugin-install:
	@echo Installing v2plugin
	$(call make-on-node1, install-shell-completion host-pluginfs-unpack \
		host-plugin-restart host-swarm-restart)

# Just like demo-v2plugin except builds are done locally and cached
demo-v2plugin-from-local: export CONTIV_DOCKER_VERSION ?= $(V2PLUGIN_DOCKER_VERSION)
demo-v2plugin-from-local: export CONTIV_DOCKER_SWARM := swarm_mode
demo-v2plugin-from-local: tar host-pluginfs-create start v2plugin-install

# demo v2plugin on VMs: creates plugin assets, starts docker swarm
# then creates and enables v2plugin
demo-v2plugin: export CONTIV_DOCKER_VERSION ?= $(V2PLUGIN_DOCKER_VERSION)
demo-v2plugin: export CONTIV_DOCKER_SWARM := swarm_mode
demo-v2plugin: node1-make-targets := host-pluginfs-create
demo-v2plugin: ssh-build make-on-node1-dep v2plugin-install

# release a v2 plugin from the VM
host-plugin-release: tar host-pluginfs-create host-pluginfs-unpack host-plugin-create
	@echo dev: pushing ${CONTIV_V2PLUGIN_NAME} to docker hub
	@echo dev: need docker login with user in contiv org
	@echo "dev:   docker login --username <username>"
	docker plugin push ${CONTIV_V2PLUGIN_NAME}

# unarchive versioned binaries to bin, usually as a helper for other targets
unarchive:
	@echo Updating bin/ with binaries versioned $(VERSION)
	tar -xf $(TAR_FILE) -C bin

# pulls netplugin binaries from build container
binaries-from-container:
	docker rm netplugin-build 2>/dev/null || :
	c_id=$$(docker create --name netplugin-build \
		 netplugin-build:$(NETPLUGIN_CONTAINER_TAG)) && \
	for f in netplugin netmaster netctl contivk8s netcontiv; do \
		docker cp $${c_id}:/go/bin/$$f bin/$$f; done && \
	docker rm $${c_id}

##########################
## Packaging and Releasing
##########################

archive:
	$(TAR) --version | grep -q GNU \
		|| (echo Please use GNU tar as \'gtar\' or \'tar\'; exit 1)
	$(TAR) --owner=0 --group=0 -jcf $(TAR_FILE) \
		-C bin netplugin netmaster netctl contivk8s netcontiv \
		-C ../scripts contrib/completion/bash/netctl get-contiv-diags

# build versioned archive of netplugin binaries
tar:
	rm -f $(TAR_FILE)
	./install/k8s/contiv/contiv-compose use-release  -i -v $(BUILD_VERSION) install/k8s/contiv/contiv-base.yaml
	$(TAR) -jcf $(TAR_FILE) -C install/k8s/contiv/ .

# GITHUB_USER and GITHUB_TOKEN are needed be set (used by github-release)
release: tar
	TAR_FILENAME=$(TAR_FILENAME) TAR_FILE=$(TAR_FILE) scripts/release.sh
