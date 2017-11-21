#!/bin/sh

### Pre-requisite on the host
# run a cluster store like etcd or consul

set -e

if [ "$log_dir" == "" ]; then
    log_dir="/var/log/contiv"
fi
mkdir -p $log_dir
BOOTUP_LOGFILE="$log_dir/plugin_bootup.log"

# Redirect stdout and stdin to BOOTUP_LOGFILE
exec 1<&-  # Close stdout
exec 2<&-  # Close stderr
exec 1<>$BOOTUP_LOGFILE  # stdout read and write to logfile instead of console
exec 2>&1  # redirect stderr to where stdout is (logfile)

mkdir -p $log_dir
mkdir -p /var/run/openvswitch
mkdir -p /etc/openvswitch

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
if [ $vxlan_port != "4789" ]; then
    vxlan_port_cfg="-vxlan-port=$vxlan_port"
fi

echo "Loading OVS" >> $BOOTUP_LOGFILE
(modprobe openvswitch) || (echo "Load ovs FAILED!!! " >> $BOOTUP_LOGFILE)

echo "  Cleaning up ovsdb files" >> $BOOTUP_LOGFILE
rm -rf /var/run/openvswitch/*
rm -rf /etc/openvswitch/conf.db
rm -rf /etc/openvswitch/.conf.db.~lock~

echo "  Creating OVS DB" >> $BOOTUP_LOGFILE
(ovsdb-tool create  /etc/openvswitch/conf.db /usr/share/openvswitch/vswitch.ovsschema) || (while true; do sleep 1; done)

echo "  Starting OVSBD server " >> $BOOTUP_LOGFILE
ovsdb-server --remote=punix:/var/run/openvswitch/db.sock --remote=db:Open_vSwitch,Open_vSwitch,manager_options --private-key=db:Open_vSwitch,SSL,private_key --certificate=db:Open_vSwitch,SSL,certificate --bootstrap-ca-cert=db:Open_vSwitch,SSL,ca_cert --log-file=$log_dir/ovs-db.log -vsyslog:dbg -vfile:dbg --pidfile --detach /etc/openvswitch/conf.db >> $BOOTUP_LOGFILE
echo "  Starting ovs-vswitchd " >> $BOOTUP_LOGFILE
ovs-vswitchd -v --pidfile --detach --log-file=$log_dir/ovs-vswitchd.log -vconsole:err -vsyslog:info -vfile:info &
ovs-vsctl set-manager tcp:127.0.0.1:6640
ovs-vsctl set-manager ptcp:6640

echo "Started OVS, logs in $log_dir" >> $BOOTUP_LOGFILE

set +e

echo "Starting Netplugin " >> $BOOTUP_LOGFILE
while true ; do
    echo "/netplugin $dbg_flag -plugin-mode=$plugin_mode $vxlan_port_cfg -vlan-if=$iflist -cluster-store=$cluster_store `if [ $cluster_tls_verify ]; then echo -db-tls-verify;fi` -db-tls-cert=$cluster_tls_cert -db-tls-key=$cluster_tls_key -db-tls-cacert=$cluster_tls_cacert $ctrl_ip_cfg=$vtep_ip_cfg" >> $BOOTUP_LOGFILE
    /netplugin $dbg_flag -plugin-mode=$plugin_mode $vxlan_port_cfg -vlan-if=$iflist -cluster-store=$cluster_store `if [ $cluster_tls_verify ]; then echo -db-tls-verify;fi` -db-tls-cert=$cluster_tls_cert -db-tls-key=$cluster_tls_key -db-tls-cacert=$cluster_tls_cacert $ctrl_ip_cfg $vtep_ip_cfg &> $log_dir/netplugin.log
    echo "CRITICAL : Net Plugin has exited, Respawn in 5" >> $BOOTUP_LOGFILE
    mv $log_dir/netplugin.log $log_dir/netplugin.log.lastrun
    sleep 5
    echo "Restarting Netplugin " >> $BOOTUP_LOGFILE
done &

if [ $plugin_role == "master" ]; then
    if [ -z "$fwd_mode" ]; then
        echo "fwd_mode is not set, plugin cannot be enabled"
        exit 1
    fi
    echo "Starting Netmaster " >> $BOOTUP_LOGFILE
    while  true ; do
        echo "/netmaster $dbg_flag -plugin-name=$plugin_name -cluster-mode=$plugin_mode -cluster-store=$cluster_store `if [ $cluster_tls_verify ]; then echo -cluster-tls-verify;fi`  -cluster-tls-cert $cluster_tls_cert -cluster-tls-key $cluster_tls_key -cluster-tls-cacert $cluster_tls_cacert $listen_url_cfg $control_url_cfg" >> $BOOTUP_LOGFILE
        /netmaster $dbg_flag -plugin-name=$plugin_name -cluster-mode=$plugin_mode -cluster-store=$cluster_store `if [ $cluster_tls_verify ]; then echo -cluster-tls-verify;fi`  -cluster-tls-cert $cluster_tls_cert -cluster-tls-key $cluster_tls_key -cluster-tls-cacert $cluster_tls_cacert $listen_url_cfg $control_url_cfg &> $log_dir/netmaster.log
        echo "CRITICAL : Net Master has exited, Respawn in 5s" >> $BOOTUP_LOGFILE
	mv $log_dir/netmaster.log $log_dir/netmaster.log.lastrun
        sleep 5
        echo "Restarting Netmaster " >> $BOOTUP_LOGFILE
    done &

    set -e
    echo "Waiting for netmaster to be ready for connections"
    # wait till netmaster starts to listen
    for i in $(seq 1 10); do
        [ "$(curl -s -o /dev/null -w '%{http_code}' $control_url)" != "000" ] \
           && break
        sleep 1
    done
    if [ "$i" -ge "10" ]; then
        echo "netmaster port not open (needed to set forwarding mode), plugin failed"
        exit 1
    fi
    sleep 1
    echo "Netmaster ready for connections, setting forward mode to $fwd_mode"
    /netctl --netmaster http://$control_url global set --fwd-mode "$fwd_mode"
    echo "Forward mode is set"
else
    echo "Not starting netmaster as plugin role is" $plugin_role >> $BOOTUP_LOGFILE
fi

while true; do sleep 1; done
