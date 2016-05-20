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
# Container image for netplugin
#
# Run netplugin:
# docker run --net=host <image> -host-label=<label>
##

FROM golang:1.5.1
MAINTAINER Madhav Puri <mapuri@cisco.com> (@mapuri)


# Insert your proxy server settings if this build is running behind 
# a proxy.
#ENV http_proxy ""
#ENV https_proxy ""
ENV GOPATH /go/

ENV NET_CONTAINER_BUILD 1

COPY ./ /go/src/github.com/contiv/netplugin/

WORKDIR /go/src/github.com/contiv/netplugin/

RUN make build

ENTRYPOINT ["netplugin"]
CMD ["--help"]
