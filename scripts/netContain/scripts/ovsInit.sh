#!/bin/bash
#Start OVS in the Contiv container

set -euo pipefail

modprobe openvswitch

mkdir -p /var/run/openvswitch
mkdir -p /var/contiv/log/

sleep 2

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
             --log-file=/var/contiv/log/ovs-db.log -vsyslog:dbg -vfile:dbg \
             --pidfile --detach /etc/openvswitch/conf.db

echo "INFO: Starting ovs-vswitchd"
ovs-vswitchd -v --pidfile --detach --log-file=/var/contiv/log/ovs-vswitchd.log \
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

exit 0
