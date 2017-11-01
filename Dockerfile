##
#Copyright 2014 Cisco Systems Inc. All rights reserved.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
##

##
# Container image for compiled netplugin binaries
#
# Run netplugin:
# docker run --net=host <image> -host-label=<label>
##

FROM golang:1.7.6

# Insert your proxy server settings if this build is running behind
# a proxy.
#ENV http_proxy ""
#ENV https_proxy ""
ARG http_proxy
ARG https_proxy

WORKDIR /go/src/github.com/contiv/netplugin/

ENTRYPOINT ["netplugin"]
CMD ["--help"]

# by far, most of the compilation time is building vendor packages
# build the vendor dependencies as a separate docker caching layer
COPY ./vendor/ /go/src/github.com/contiv/netplugin/vendor/
COPY ./objdb/ /go/src/github.com/contiv/netplugin/objdb/

RUN GOGC=1500 go install -ldflags "-s -w" $(go list ./vendor/...)

# build the netplugin binaries
COPY ./ /go/src/github.com/contiv/netplugin/

ARG BUILD_VERSION=""
ARG NIGHTLY_RELEASE=""

RUN GOPATH=/go/ \
    BUILD_VERSION="${BUILD_VERSION}" \
    NIGHTLY_RELEASE="${NIGHTLY_RELEASE}" \
    make compile \
      && cp scripts/contrib/completion/bash/netctl /etc/bash_completion.d/netctl \
      && netplugin -version
