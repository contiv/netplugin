#!/bin/bash

# This script is used by Jenkins CI scripts to setup commn environment

#$WORKSPACE points to the Jenkins' workspace root
export GOPATH=$WORKSPACE
export GOBIN=$GOPATH/bin
export GOSRC=$GOPATH/src
export PATH=$PATH:/sbin/:/usr/local/go/bin:$GOBIN:$GOPATH/src/github.com/contiv/netplugin/bin
