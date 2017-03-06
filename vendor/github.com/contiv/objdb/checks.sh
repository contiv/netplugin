#!/bin/sh

set -e

dirs=$(go list ./... | sed -e 's!github.com/contiv/objdb!.!g' | grep -v ./vendor)
files=$(find . -type f -name '*.go' | grep -v ./vendor)

echo "Running gofmt..."
set +e
out=$(gofmt -s -l ${files})
set -e
# gofmt can include empty lines in its output
if [ "$(echo $out | sed '/^$/d' | wc -l)" -gt 0 ]; then
	echo 1>&2 "gofmt errors in:"
	echo 1>&2 "${out}"
	exit 1
fi

echo "Running ineffassign..."
[ -n "$(which ineffassign)" ] || go get github.com/gordonklaus/ineffassign
for i in ${dirs}; do
	ineffassign $i
done

echo "Running golint..."
[ -n "$(which golint)" ] || go get github.com/golang/lint/golint
set +e
out=$(golint ./... | grep -vE '^vendor')
set -e
if [ "$(echo $out | sed '/^$/d' | wc -l)" -gt 0 ]; then
	echo 1>&2 "golint errors in:"
	echo 1>&2 "${out}"
	exit 1
fi

echo "Running govet..."
set +e
out=$(go tool vet -composites=false ${dirs} 2>&1 | grep -v vendor)
set -e

if [ "$(echo $out | sed '/^$/d' | wc -l)" -gt 0 ]; then
	echo 1>&2 "go vet errors in:"
	echo 1>&2 "${out}"
	exit 1
fi

echo "Running gocyclo..."
[ -n "$(which gocyclo)" ] || go get github.com/fzipp/gocyclo
set +e
out=$(gocyclo -over 20 . | grep -v vendor)
set -e
if [ "$(echo $out | sed '/^$/d' | wc -l)" -gt 0 ]; then
	echo 1>&2 "gocycle errors in:"
	echo 1>&2 "${out}"
	exit 1
fi

echo "Running misspell..."
[ -n "$(which misspell)" ] || go get github.com/client9/misspell/...
set +e
out=$(misspell -locale US -error -i exportfs ${files})
set -e
if [ "$(echo $out | sed '/^$/d' | wc -l)" -gt 0 ]; then
	echo 1>&2 "misspell errors in:"
	echo 1>&2 "${out}"
	exit 1
fi

echo "Running deadcode..."
[ -n "$(which deadcode)" ] || go get github.com/remyoudompheng/go-misc/deadcode/...
set +e
out=$(deadcode ${dirs} 2>&1)
set -e
if [ "$(echo $out | sed '/^$/d' | wc -l)" -gt 0 ]; then
	echo 1>&2 "deadcode errors in:"
	echo 1>&2 "${out}"
	exit 1
fi
