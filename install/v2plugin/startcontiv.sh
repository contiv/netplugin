#!/bin/bash

### Pre-requisite on the host
# run a cluster store like etcd or consul

set -e
echo "INFO: Starting contiv net with ARGS:"
echo "$@"
echo "INFO: Starting contiv net with ENV:"
/usr/bin/env | grep CONTIV_

# this is different between k8s and v2plugin because v2plugin have netmaster
# in one container
if [ -z "$CONTIV_ROLE" ]; then
    CONTIV_ROLE="netplugin"
elif [ "$CONTIV_ROLE" != "netmaster" ] && [ "$CONTIV_ROLE" != "netplugin" ]; then
    echo "CRITICAL: ENV CONTIV_ROLE must be in [netmaster, netplugin]"
    echo "CRITICAL: Unknown contiv role"
    exit 1
fi
echo "INFO: Starting contiv net as role: $CONTIV_ROLE"

# setting up logs
if [ -z "$CONTIV_LOG_DIR" ]; then
    CONTIV_LOG_DIR="/var/log/contiv"
fi
mkdir -p "$CONTIV_LOG_DIR"
echo "INFO: Logging contiv net under: $CONTIV_LOG_DIR"

BOOTUP_LOGFILE="$CONTIV_LOG_DIR/plugin_bootup.log"
# Redirect stdout and stdin to BOOTUP_LOGFILE
exec 1<&-  # Close stdout
exec 2<&-  # Close stderr
exec 1<>$BOOTUP_LOGFILE  # stdout read and write to logfile instead of console
exec 2>&1  # redirect stderr to where stdout is (logfile)

mkdir -p "$CONTIV_LOG_DIR" /var/run/openvswitch /etc/openvswitch

# setting up ovs
# TODO: this is the same code in ovsInit.sh, needs to reduce the duplication
set -uo pipefail

modprobe openvswitch || (echo "CRITICAL: Failed to load kernel module openvswitch" && exit 1 )
echo "INFO: Loaded kernel module openvswitch"

if [ -d "/etc/openvswitch" ]; then
    if [ -f "/etc/openvswitch/conf.db" ]; then
        echo "INFO: The Open vSwitch database exists"
    else
        echo "INFO: The Open VSwitch database doesn't exist"
        echo "INFO: Creating the Open VSwitch database..."
        ovsdb-tool create /etc/openvswitch/conf.db /usr/share/openvswitch/vswitch.ovsschema
    fi
else
    echo "CRITICAL: Open vSwitch is not mounted from host"
    exit 1
fi

echo "INFO: Starting ovsdb-server..."
ovsdb-server --remote=punix:/var/run/openvswitch/db.sock \
             --remote=db:Open_vSwitch,Open_vSwitch,manager_options \
             --private-key=db:Open_vSwitch,SSL,private_key \
             --certificate=db:Open_vSwitch,SSL,certificate \
             --bootstrap-ca-cert=db:Open_vSwitch,SSL,ca_cert \
             --log-file=$CONTIV_LOG_DIR/ovs-db.log -vsyslog:info -vfile:info \
             --pidfile --detach /etc/openvswitch/conf.db

echo "INFO: Starting ovs-vswitchd"
ovs-vswitchd -v --pidfile --detach --log-file=$CONTIV_LOG_DIR/ovs-vswitchd.log \
    -vconsole:err -vsyslog:info -vfile:info &

retry=0
while [[ $(ovsdb-client list-dbs | grep -c Open_vSwitch) -eq 0 ]] ; do
    if [[ ${retry} -eq 5 ]]; then
        echo "CRITICAL: Failed to start ovsdb in 5 seconds."
        exit 1
    else
        echo "INFO: Waiting for ovsdb to start..."
        sleep 1
        ((retry+=1))
    fi
done

echo "INFO: Setting OVS manager (tcp)..."
ovs-vsctl set-manager tcp:127.0.0.1:6640

echo "INFO: Setting OVS manager (ptcp)..."
ovs-vsctl set-manager ptcp:6640

# starting services
set +e
if [ "$CONTIV_ROLE" = "netmaster" ]; then
    while  true ; do
        echo "INFO: Starting contiv netmaster"
        set -x
        /contiv/bin/netmaster "$@" &>> "$CONTIV_LOG_DIR/netmaster.log"
        set +x
        echo "ERROR: Contiv netmaster has exited, restarting in 5s"
        sleep 5
    done &
fi

while true ; do
    echo "INFO: Starting contiv netplugin"
    set -x
    /contiv/bin/netplugin "$@" &>> "$CONTIV_LOG_DIR/netplugin.log"
    set +x
    echo "ERROR: Contiv netplugin has exited, restarting in 5s"
    sleep 5
done
