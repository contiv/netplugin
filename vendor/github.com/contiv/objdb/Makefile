
.PHONY: all build update start stop test host-build host-test godep

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

# build compiles and installs the code after running code quality checks
host-build:
	./checks.sh 
	go get github.com/tools/godep
	go install ./ ./modeldb

host-test:
	etcdctl rm --recursive /contiv.io || true
	go test -v ./ ./modeldb
	go test -bench=. -run "Benchmark"

# godep updates Godeps/Godeps.json
godep:
	godep save ./...
