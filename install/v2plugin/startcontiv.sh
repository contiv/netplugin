#!/bin/sh

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
if [ $control_url != ":9999" ]; then
    control_url_cfg="-control-url=$control_url"
fi

echo "Loading OVS" >> $BOOTUP_LOGFILE
(modprobe openvswitch) || (echo "Load ovs FAILED!!! " >> $BOOTUP_LOGFILE && while true; do sleep 1; done)

echo "  Cleaning up ovsdb files" >> $BOOTUP_LOGFILE
rm -rf /var/run/openvswitch/*
rm -rf /etc/openvswitch/conf.db
rm -rf /etc/openvswitch/.conf.db.~lock~

echo "  Creating OVS DB" >> $BOOTUP_LOGFILE
(ovsdb-tool create  /etc/openvswitch/conf.db /usr/share/openvswitch/vswitch.ovsschema) || (while true; do sleep 1; done)

echo "  Starting OVSBD server " >> $BOOTUP_LOGFILE
ovsdb-server --remote=punix:/var/run/openvswitch/db.sock --remote=db:Open_vSwitch,Open_vSwitch,manager_options --private-key=db:Open_vSwitch,SSL,private_key --certificate=db:Open_vSwitch,SSL,certificate --bootstrap-ca-cert=db:Open_vSwitch,SSL,ca_cert --log-file=/var/log/openvswitch/ovs-db.log -vsyslog:dbg -vfile:dbg --pidfile --detach /etc/openvswitch/conf.db >> $BOOTUP_LOGFILE
echo "  Starting ovs-vswitchd " >> $BOOTUP_LOGFILE
ovs-vswitchd -v --pidfile --detach --log-file=/var/log/openvswitch/ovs-vswitchd.log -vconsole:err -vsyslog:info -vfile:info &
ovs-vsctl set-manager tcp:127.0.0.1:6640 
ovs-vsctl set-manager ptcp:6640

echo "Started OVS, logs in /var/log/openvswitch/" >> $BOOTUP_LOGFILE

echo "Starting Netplugin " >> $BOOTUP_LOGFILE
while true ; do
    echo "/netplugin $dbg_flag -plugin-mode docker -vlan-if $iflist -cluster-store $cluster_store $ctrl_ip_cfg $vtep_ip_cfg" >> $BOOTUP_LOGFILE
    /netplugin $dbg_flag -plugin-mode docker -vlan-if $iflist -cluster-store $cluster_store $ctrl_ip_cfg $vtep_ip_cfg &> /var/run/contiv/log/netplugin.log
    echo "CRITICAL : Netplugin has exited, Respawn in 5s" >> $BOOTUP_LOGFILE
    sleep 5
    echo "Restarting Netplugin " >> $BOOTUP_LOGFILE
done &

if [ $plugin_role == "master" ]; then
    echo "Starting Netmaster " >> $BOOTUP_LOGFILE
    while  true ; do
        echo "/netmaster $dbg_flag -plugin-name=$plugin_name -cluster-store=$cluster_store $listen_url_cfg $control_url_cfg " >> $BOOTUP_LOGFILE
        /netmaster $dbg_flag -plugin-name=$plugin_name -cluster-store=$cluster_store $listen_url_cfg $control_url_cfg &> /var/run/contiv/log/netmaster.log
        echo "CRITICAL : Netmaster has exited, Respawn in 5s" >> $BOOTUP_LOGFILE
        echo "Restarting Netmaster " >> $BOOTUP_LOGFILE
        sleep 5
    done &
else
    echo "Not starting netmaster as plugin role is" $plugin_role >> $BOOTUP_LOGFILE
fi

while true; do sleep 1; done

