#!/bin/bash
#Initialize complete contiv container. Start OVS and Net Plugin

ARG1=${1:-none}


plugin="docker"
vtep_ip=""
fwd_mode="bridge"
cstore=""
netmaster=false

while getopts ":mp:v:i:f:c:" opt; do
    case $opt in
       m) 
          netmaster=true
          ;;
       v)
          vtep_ip = $OPTARG
          netplugin=true
          ;;
       f)
          fwd_mode = $OPTARG
          netplugin=true
          ;;
       c)
          cstore = $OPTARG
          ;; 
       p) 
          plugin=$OPTARG
          netplugin=true
          ;;
       :)
          echo "An argument required for $OPTARG was not passed"
          ;;
       ?)
          echo "Invalid option supplied"
          ;;
     esac
done

if [ $netplugin == false && $netmaster == false ]; then
   echo "Neither Netmaster or Net Plugin Options Specificed"
   exit
fi

if [ $ARG1 == "reinit" ]; then
    ovs-vsctl del-br contivVlanBridge
    ovs-vsctl del-br contivVxlanBridge
fi

/contiv/scripts/ovsInit.sh


if [ $netmaster ]; then
   echo "Starting Net Master "
   while [ true ]; do
       if [ $cstore != "" ]; then
           /contiv/bin/netmaster -cluster-store $cstore &> /var/log/contiv/netmaster.log
       else
           /contiv/bin/netmaster &> /var/log/contiv/netmaster.log
       fi
       echo "CRITICAL : Net Master has exited, Respawn in 5"
       sleep 5
   done &
fi
   
if [ $netplugin == true ]; then
   if [ $vtep_ip == "" || $vlan_if == "" ]; then 
       echo "Net Plugin Cannot be started without specifying the VETP or VLAN Interface"
       exit
   fi
   while [ true ]; do
       if [ $cstore != "" ]; then
           /contiv/bin/netplugin -cluster-store $cstore  -vtep-ip $vtep_ip -vlan-if $vlan_if -fwd-mode $fwdmode -plugin-mode $plugin &> /var/log/contiv/netplugin.log
       else
           /contiv/bin/netplugin -vtep-ip $vtep_ip -vlan-if $vlan_if -fwd-mode $fwd_mode -plugin-mode $plugin &> /var/log/contiv/netplugin.log
       fi
       echo "CRITICAL : Net Plugin has exited, Respawn in 5"
       sleep 5
   done &
fi


while true; do sleep 1; done
