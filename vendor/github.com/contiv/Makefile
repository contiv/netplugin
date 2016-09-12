
.PHONY: all build test

all: build test

build: start
	vagrant ssh node1 -c 'cd /opt/gopath/src/github.com/contiv/ofnet && make host-build'

update:
	vagrant box update

start: update
	vagrant up

stop:
	vagrant destroy -f

test: start
	vagrant ssh node1 -c 'cd /opt/gopath/src/github.com/contiv/ofnet && make host-test'

host-build:
	./checks "./*.go ./ofctrl/ ./ovsdbDriver/ ./pqueue/ ./rpcHub/"
	go get github.com/tools/godep
	godep go install ./ ./ofctrl

host-test:
	sudo -E PATH=$(PATH) /opt/gopath/bin/godep go test -v ./
	sudo -E PATH=$(PATH) /opt/gopath/bin/godep go test -v ./ofctrl
	sudo -E PATH=$(PATH) /opt/gopath/bin/godep go test -v ./pqueue
