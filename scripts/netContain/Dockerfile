##
#Copyright 2017 Cisco Systems Inc. All rights reserved.
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

FROM ubuntu:16.04

# Make sure to Modify the Proxy Server values if required 
# ENV export http_proxy=http://proxy.localhost.com:8080
# ENV export https_proxy=https://proxy.localhost.com:8080

RUN apt-get update \
 && apt-get install -y openvswitch-switch=2.5.2* \
        net-tools \
        iptables \
 && rm -rf /var/lib/apt/lists/*

COPY ./bin /contiv/bin/
COPY ./scripts /contiv/scripts/

ENTRYPOINT ["/contiv/scripts/contivNet.sh"]
