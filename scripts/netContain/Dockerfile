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

# One Container for OVS / netplugin / netmaster 

FROM ubuntu:14.04

MAINTAINER Rajesh Nataraja <rajenata@cisco.com>

# Make sure to Modify the Proxy Server values if required 
# ENV export http_proxy=http://proxy.localhost.com:8080
# ENV export https_proxy=https://proxy.localhost.com:8080


#Install the Open vSwitch package on this ubuntu container.
RUN apt update
RUN apt --assume-yes  install openvswitch-switch

#Copy the Net Plugin Binaries  and the Scripts required to 
# start the Net Plugin and Net Master. 

RUN mkdir -p /contiv/bin
RUN mkdir -p /contiv/scripts

COPY ./netplugin /contiv/bin/
COPY ./netmaster /contiv/bin/
COPY ./ovsInit.sh /contiv/scripts/
COPY ./contivInit.sh /contiv/scripts/
COPY ./contivNet.sh /contiv/scripts/
COPY ./contivRc /contiv/scripts/


ENTRYPOINT ["/contiv/scripts/contivNet.sh"]
