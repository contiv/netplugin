# builder is where netplugin got complied
FROM golang:1.7.6 as builder

ENV GOPATH /go

COPY . $GOPATH/src/github.com/contiv/netplugin/

WORKDIR $GOPATH/src/github.com/contiv/netplugin/

RUN VERSION_SUFFIX="$(if git diff-index --quiet HEAD --; then echo '-unsupported'; fi)" && \
    GIT_COMMIT=$(git rev-parse --short HEAD)$VERSION_SUFFIX && \
    BUILD_VERSION=$(git describe --tags --always)$VERSION_SUFFIX  && \
    PKG_NAME=github.com/contiv/netplugin/version && \
    BUILD_TIME=$(date -u +%m-%d-%Y.%H-%M-%S.UTC) && \
    GOGC=1500 CGO_ENABLED=0 go install -v \
    -a -installsuffix cgo \
    -ldflags "-X $PKG_NAME.version=$BUILD_VERSION \
    -X $PKG_NAME.buildTime=$BUILD_TIME \
    -X $PKG_NAME.gitCommit=$GIT_COMMIT \
    -s -w -d" -pkgdir /tmp/foo-cgo \
    ./netplugin/ ./netmaster/ ./netctl/netctl/ ./mgmtfn/k8splugin/contivk8s/ && \
    mkdir -p /contiv/bin && \
    for bin in netplugin netmaster netctl contivk8s; do cp /go/bin/$bin /contiv/bin/ ; done && \
    /contiv/bin/netplugin --version && /contiv/bin/netmaster --version

# The container where netplugin will be run
FROM ubuntu:16.04

RUN apt-get update \
 && apt-get install -y openvswitch-switch=2.5.2* \
        net-tools \
        iptables \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /contiv/bin/ /contiv/bin/
COPY --from=builder /go/src/github.com/contiv/netplugin/scripts/netContain/scripts/ /contiv/scripts/

ENTRYPOINT ["/contiv/scripts/contivNet.sh"]
