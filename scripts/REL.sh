#!/bin/bash

# This script is called from the Jenkins CI after the push sanity succeeds.
# It shall publish a new release on github with the changes pushed.

. $(dirname $0)/env.sh
cd $GOSRC/github.com/contiv/netplugin
make release
