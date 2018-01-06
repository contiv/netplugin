#!/bin/bash
#Initialize contiv container. Start OVS and netplugin
set -e

echo "INFO: Starting contiv net with ARGS:"
echo "$@"
echo "INFO: Starting contiv net with ENV:"
/usr/bin/env | grep CONTIV_

# These files indicate if the netmaster/netplugin process needs to be restarted
touch /tmp/restart_netmaster
touch /tmp/restart_netplugin

#This needs to be fixed, we cant rely on the value being supplied from
# parameters, just explosion of parameters is not a great solution
#export no_proxy="0.0.0.0, 172.28.11.253"
#echo "172.28.11.253 netmaster" > /etc/hosts

#Needed for netplugin to connect with OVS, This needs to be
#fixed as well. netplugin should have OVS locally.
echo "0.0.0.0 localhost" >> /etc/hosts

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

set -uo pipefail

mkdir -p /opt/contiv/ /var/log/contiv

if [ -d /var/contiv/log ]; then
    # /var/contiv/log/ is deprecated, move all data to /var/log/contiv
    cp -a /var/contiv/log/* /var/log/contiv/
    echo "INFO: Copied contiv log from /var/contiv/log (deprecated) to /var/log/contiv"
fi

if [ "$CONTIV_MODE" = "kubernetes" ]; then
    echo "INFO: Setting kubernetes configs"
    mkdir -p /opt/contiv/config
    mkdir -p /var/contiv/config
    echo ${CONTIV_K8S_CONFIG} > /var/contiv/config/contiv.json
    set -x
    cp /var/contiv/config/contiv.json /opt/contiv/config/contiv.json
    set +x
    if [ "$CONTIV_ROLE" = "netplugin" ]; then
        mkdir -p /opt/cni/bin
        cp /contiv/bin/contivk8s /opt/cni/bin/
        mkdir -p /etc/cni/net.d/
        set -x
        echo ${CONTIV_CNI_CONFIG} > /etc/cni/net.d/1-contiv.conf
        set +x
    fi
fi

set +e
if [ "$CONTIV_ROLE" = "netmaster" ]; then
    while true; do
        echo "INFO: Starting contiv netmaster"
        if [ -f /tmp/restart_netmaster ]; then
            set -x
            /contiv/bin/netmaster "$@"
            set +x
            echo "ERROR: Contiv netmaster has exited, restarting in 5s"
        fi
        sleep 5
    done
elif [ "$CONTIV_ROLE" = "netplugin" ]; then
    while true; do
        echo "INFO: Starting contiv netplugin"
        if [ -f /tmp/restart_netplugin ]; then
            set -x
            /contiv/bin/netplugin "$@"
            set +x
            echo "ERROR: Contiv netplugin has exited, restarting in 5s"
        fi
        sleep 5
    done
fi
