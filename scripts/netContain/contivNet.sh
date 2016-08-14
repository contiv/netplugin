#!/bin/bash
#Initialize complete contiv container. Start OVS and Net Plugin

reinit=false
plugin="docker"
vtep_ip=""
fwd_mode="bridge"
cstore=""
cmode="bridge"
netmaster=false
netplugin=false
vlan_if=""

#This needs to be fixed, we cant rely on the value being supplied from 
# paramters, just explosion of parameters is not a great solution
#export no_proxy="0.0.0.0, 172.28.11.253" 
#echo "172.28.11.253 netmaster" > /etc/hosts

#Needed for Net Plugin to connect with OVS, This needs to be 
#fixed as well. netplugin should have OVS locally. 
echo "0.0.0.0 localhost" >> /etc/hosts

while getopts ":xmp:v:i:f:c:" opt; do
    case $opt in
       m) 
          netmaster=true
          ;;
       v)
          vtep_ip=$OPTARG
          netplugin=true
          ;;
       i)
          vlan_if=$OPTARG
          netplugin=true
          ;;
       f)
          fwd_mode=$OPTARG
          netplugin=true
          ;;
       c)
          cstore=$OPTARG
          ;; 
       p) 
          plugin=$OPTARG
          ;;
       x)
          reinit=true
          ;;
       :)
          echo "An argument required for $OPTARG was not passed"
          ;;
       ?)
          echo "Invalid option supplied"
          ;;
     esac
done

if [ $netplugin == false ] && [ $netmaster == false ]; then
   echo "Neither Netmaster or Net Plugin Options Specificed"
   exit
fi


if [ $netplugin == true ]; then
    /contiv/scripts/ovsInit.sh
fi

if [ $reinit == true ]; then
    ovs-vsctl del-br contivVlanBridge
    ovs-vsctl del-br contivVxlanBridge
fi


mkdir -p /opt/contiv/

if  [ "$plugin" == "kubernetes" ]; then
    mkdir -p  /opt/contiv/config
    cp /var/contiv/config/contiv.json /opt/contiv/config/contiv.json
fi

if [ $netmaster == true ]; then
   echo "Starting Netmaster "
   while [ true ]; do
       if [ "$cstore" != "" ]; then
           /contiv/bin/netmaster  -cluster-mode $plugin -cluster-store $cstore &> /var/contiv/log/netmaster.log
       else
           /contiv/bin/netmaster -cluster-mode $plugin  &> /var/contiv/log/netmaster.log
       fi
       echo "CRITICAL : Net Master has exited, Respawn in 5"
       sleep 5
   done &
fi
   
if [ $netplugin == true ]; then


   if [ "$vtep_ip" == "" ] || [ "$vlan_if" == "" ]; then 
       echo "Net Plugin Cannot be started without specifying the VETP or VLAN Interface"
       exit
   fi

   while [ true ]; do
       if [ "$cstore" != "" ]; then
           /contiv/bin/netplugin -cluster-store $cstore  -vtep-ip $vtep_ip -vlan-if $vlan_if -fwd-mode $fwd_mode -plugin-mode $plugin &> /var/contiv/log/netplugin.log
       else
           /contiv/bin/netplugin -vtep-ip $vtep_ip -vlan-if $vlan_if -fwd-mode $fwd_mode -plugin-mode $plugin &> /var/contiv/log/netplugin.log
       fi
       echo "CRITICAL : Net Plugin has exited, Respawn in 5"
       sleep 5
   done &
fi


while true; do sleep 1; done
