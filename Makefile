
.PHONY: all build test

all: build test

build: start
	vagrant ssh node1 -c 'cd /opt/gopath/src/github.com/contiv/objdb && make host-build'

update:
	vagrant box update

start: update
	vagrant up

stop:
	vagrant destroy -f

test: start build
	vagrant ssh node1 -c 'cd /opt/gopath/src/github.com/contiv/objdb && make host-test'

host-build:
	./checks "./*.go ./modeldb"
	go get github.com/tools/godep
	godep go install ./ ./modeldb

host-test:
	godep go test -v ./ ./modeldb
	godep go test -bench=. -run "Benchmark"
