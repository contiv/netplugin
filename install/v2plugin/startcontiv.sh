#!/bin/bash

set -e 

### Pre-requisite on the host
# run a cluster store like etcd or consul

mkdir -p /var/run/contiv/log
mkdir -p /var/run/openvswitch
mkdir -p /etc/openvswitch

BOOTUP_LOGFILE="/var/run/contiv/log/plugin_bootup.log"

echo "V2 Plugin logs" > $BOOTUP_LOGFILE

if [ $iflist == "" ]; then
    echo "iflist is empty. Host interface(s) should be specified to use vlan mode" >> $BOOTUP_LOGFILE
fi
if [ $ctrl_ip != "none" ]; then
    ctrl_ip_cfg="-ctrl-ip=$ctrl_ip"
fi
if [ $vtep_ip != "none" ]; then
    vtep_ip_cfg="-vtep-ip=$vtep_ip"
fi
if [ $listen_url != ":9999" ]; then
    listen_url_cfg="-listen-url=$listen_url"
fi

echo "Loading OVS" >> $BOOTUP_LOGFILE
(modprobe openvswitch) || (echo "Load ovs FAILED!!! " >> $BOOTUP_LOGFILE && while true; do sleep 1; done)

echo "Starting OVS" >> $BOOTUP_LOGFILE
/usr/share/openvswitch/scripts/ovs-ctl restart --system-id=random --with-logdir=/var/run/contiv/log

echo "Starting Netplugin " >> $BOOTUP_LOGFILE
netplugin_retry="0"
while [ true ]; do
    echo "/netplugin $dbg_flag -plugin-mode docker -vlan-if $iflist -cluster-store $cluster_store $ctrl_ip_cfg $vtep_ip_cfg" >> $BOOTUP_LOGFILE
    /netplugin $dbg_flag -plugin-mode docker -vlan-if $iflist -cluster-store $cluster_store $ctrl_ip_cfg $vtep_ip_cfg &>> /var/run/contiv/log/netplugin.log
    ((netplugin_retry++))
    if [ $netplugin_retry == "10" ] ; then 
        echo "Giving up after $netplugin_retry retries" >> $BOOTUP_LOGFILE
        exit
    fi
    echo "CRITICAL : Net Plugin has exited, Respawn in 5" >> $BOOTUP_LOGFILE
    sleep 5
    echo "Restarting Netplugin " >> $BOOTUP_LOGFILE
done &

if [ $plugin_role == "master" ]; then
    echo "Starting Netmaster " >> $BOOTUP_LOGFILE
    netmaster_retry=0
    while [ true ]; do
        echo "/netmaster $dbg_flag -plugin-name=$plugin_name -cluster-store=$cluster_store $listen_url_cfg " >> $BOOTUP_LOGFILE
        /netmaster $dbg_flag -plugin-name=$plugin_name -cluster-store=$cluster_store $listen_url_cfg &>> /var/run/contiv/log/netmaster.log
        ((netmaster_retry++))
        if [ $netmaster_retry == "10" ] ; then 
            echo "Giving up after $netmaster_retry retries" >> $BOOTUP_LOGFILE
            exit
        fi
        echo "CRITICAL : Net Master has exited, Respawn in 5" >> $BOOTUP_LOGFILE
        echo "Restarting Netmaster " >> $BOOTUP_LOGFILE
        sleep 5
    done &
else
    echo "Not starting netmaster as plugin role is" $plugin_role >> $BOOTUP_LOGFILE
fi

while true; do sleep 1; done

