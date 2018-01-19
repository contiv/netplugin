#!/bin/bash
#Initialize contiv container. Start OVS and netplugin
set -ex

echo "INFO: Starting contiv net with ARGS:"
echo "$@"
echo "INFO: Starting contiv net with ENV:"
/usr/bin/env | grep CONTIV_

if [ -z "$CONTIV_ROLE" ]; then
    echo "CRITICAL: ENV CONTIV_ROLE must be set"
    echo "CRITICAL: Unknown contiv role"
    exit 1
elif [ "$CONTIV_ROLE" != "netmaster" ] && [ "$CONTIV_ROLE" != "netplugin" ]; then
    echo "CRITICAL: ENV CONTIV_ROLE must be in [netmaster, netplugin]"
    echo "CRITICAL: Unknown contiv role"
    exit 1
fi
echo "INFO: Running contiv as $CONTIV_ROLE"

if [ ! -z "$CONTIV_MODE" ]; then
    if [ "$CONTIV_ROLE" = "netmaster" ] && [ -z "$CONTIV_NETMASTER_MODE" ] ; then
        CONTIV_NETMASTER_MODE="$CONTIV_ROLE"
    elif [ "$CONTIV_ROLE" = "netplugin" ] && [ -z "$CONTIV_NETPLUGIN_MODE" ] ; then
        CONTIV_NETPLUGIN_MODE="$CONTIV_ROLE"
    fi
elif [ ! -z "$CONTIV_NETMASTER_MODE" ]; then
    CONTIV_MODE="$CONTIV_NETMASTER_MODE"
elif [ ! -z "$CONTIV_NETPLUGIN_MODE" ]; then
    CONTIV_MODE="$CONTIV_NETPLUGIN_MODE"
else
    echo "CRITICAL: ENV CONTIV_MODE or CONTIV_NETMASTER_MODE or CONTIV_NETPLUGIN_MODE must be set"
    echo "CRITICAL: Unknown contiv mode"
    exit 1
fi
echo "INFO: Running contiv in mode $CONTIV_MODE"

set -ueo pipefail

if [ "$CONTIV_ROLE" = "netmaster" ]; then
    echo "INFO: Starting contiv netmaster"
    /contiv/bin/netmaster $@
elif [ "$CONTIV_ROLE" = "netplugin" ]; then
    echo "INFO: Starting contiv netplugin"
    /contiv/bin/netplugin $@
fi
echo "ERROR: Contiv $CONTIV_ROLE has exited with $?"
