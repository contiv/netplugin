#!/bin/bash

# This script is called from the Jenkins CI plugin on following github
# triggers:
# - push to master
# - pull request on master

#$WORKSPACE points to the Jenkins' workspace root
export GOPATH=$WORKSPACE
export GOBIN=$GOPATH/bin
export GOSRC=$GOPATH/src
export PATH=$PATH:/sbin/:/usr/local/go/bin:$GOBIN

cd $GOSRC/github.com/contiv/netplugin
make
