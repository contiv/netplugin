.PHONY: all build update test start stop host-build host-test

all: build test

build: start
	vagrant ssh node1 -c 'cd /opt/gopath/src/github.com/contiv/ofnet && make host-build'

update:
	vagrant box update

start: update
	vagrant up

stop:
	vagrant destroy -f

test: build 
	vagrant ssh node1 -c 'cd /opt/gopath/src/github.com/contiv/ofnet && make host-test'

host-build:
	./checks "./*.go ./libpkt/ ./ofctrl/ ./ovsdbDriver/ ./ovsSwitch/ ./pqueue/ ./rpcHub/"
	go get github.com/tools/godep
	go install ./ ./ofctrl

host-test:
	PATH=${PATH} sudo -E /usr/local/go/bin/go test -v ./
	PATH=${PATH} sudo -E /usr/local/go/bin/go test -v ./ofctrl
	PATH=${PATH} sudo -E /usr/local/go/bin/go test -v ./pqueue
