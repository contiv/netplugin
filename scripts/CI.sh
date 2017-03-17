#!/bin/bash

# This script is called from the Jenkins CI plugin on following github
# triggers:
# - push to master
# - pull request on master

. $(dirname $0)/env.sh
cd $GOSRC/github.com/contiv/netplugin
make all-CI
