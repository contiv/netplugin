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
# Container image for the powerstrip adapter of netplugin
#
# Run pslibnet:
# docker run <image> -host-label=<label> --etcd-url=<etcd-client-url, eg. http://1.2.3.4:2379>
##

FROM golang:1.4
MAINTAINER Madhav Puri <mapuri@cisco.com> (@mapuri)

ENV GOPATH /go/

COPY ./ /go/src/github.com/contiv/pslibnet

WORKDIR /go/src/github.com/contiv/pslibnet

RUN go get -d .
RUN go install -v .

# build and install netdcli as pslibnet needs it.
# Note: the following build requires the netdcli code downloaded
# by above build
WORKDIR /go/src/github.com/contiv/netplugin
RUN go install ./netdcli/

ENTRYPOINT ["pslibnet"]
CMD ["--help"]
