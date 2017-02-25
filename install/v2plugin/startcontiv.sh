#!/bin/bash
#Initialize complete contiv container. Start OVS and Net Plugin

set -e 

### Pre-requisite on the host
# etcd 
# sudo modprobe openvswitch
# sudo mkdir -p /var/contiv

mkdir -p /var/run/contiv/log
mkdir -p /var/run/openvswitch
mkdir -p /etc/openvswitch

echo "V2 Plugin logs" > /var/run/contiv/log/plugin_bootup.log

echo "Loading OVS" >> /var/run/contiv/log/plugin_bootup.log
(modprobe openvswitch) || (echo "Load ovs FAILED!!! run 'sudo modprobe openvswitch' from the host before re-enabling the plugin" >> /var/run/contiv/log/plugin_bootup.log && while true; do sleep 1; done)

echo "Starting OVS" >> /var/run/contiv/log/plugin_bootup.log
/usr/share/openvswitch/scripts/ovs-ctl restart --system-id=random --with-logdir=/var/run/contiv/log

echo "Starting Netplugin " >> /var/run/contiv/log/plugin_bootup.log
set netplugin_retry='0'
while [ true ]; do
    echo "Redirecting Netplugin logs" >> /var/run/contiv/log/plugin_bootup.log
    /netplugin $dbg_flag -plugin-mode docker -vlan-if $iflist -cluster-store $cluster_store &> /var/run/contiv/log/netplugin.log.$netplugin_retry
    echo "CRITICAL : Net Plugin has exited, Respawn in 5" >> /var/run/contiv/log/plugin_bootup.log
    ((netplugin_retry++))
    sleep 5
done &

echo "Starting Netmaster " >> /var/run/contiv/log/plugin_bootup.log
set netmaster_retry=0
while [ true ]; do
    echo "Redirecting Netmaster logs" >> /var/run/contiv/log/plugin_bootup.log
    /netmaster $dbg_flag -plugin-name=$plugin_name -cluster-store=$cluster_store &> /var/run/contiv/log/netmaster.log.$netmaster_retry
    ((netmaster_retry++))
    echo "CRITICAL : Net Master has exited, Respawn in 5" >> /var/run/contiv/log/plugin_bootup.log
    sleep 5
done &

#until pids=$(pgrep netmaster); do  sleep 1 ; done
#(/netctl global set -p netplugin) || (echo "failed to reset plugin_name" >> /var/run/contiv/log/plugin_bootup.log)
#(/netctl global set -p $plugin_name) || (echo "failed to set plugin_name to $plugin_name" >> /var/run/contiv/log/plugin_bootup.log)

while true; do sleep 1; done

