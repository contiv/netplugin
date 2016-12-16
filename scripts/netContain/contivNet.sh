#!/bin/bash
#Initialize complete contiv container. Start OVS and Net Plugin

reinit=false
plugin="docker"
vtep_ip="$VTEP_IP"
fwd_mode="bridge"
cstore="$CONTIV_ETCD"
cmode="bridge"
netmaster=false
netplugin=true
vlan_if="$VLAN_IF"

#This needs to be fixed, we cant rely on the value being supplied from 
# parameters, just explosion of parameters is not a great solution
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
    mkdir -p /opt/cni/bin
    cp /contiv/bin/contivk8s /opt/cni/bin/
    mkdir -p  /opt/contiv/config
    mkdir -p /var/contiv/config
    echo ${CONTIV_CONFIG} > /var/contiv/config/contiv.json 
    cp /var/contiv/config/contiv.json /opt/contiv/config/contiv.json
    mkdir -p /etc/cni/net.d/
    echo ${CONTIV_CNI_CONFIG} > /etc/cni/net.d/1-contiv.conf
fi

if [ $netmaster == true ]; then
   echo "Starting Netmaster "
   mkdir -p /var/contiv/log/
   while [ true ]; do
       if [ "$cstore" != "" ]; then
           /contiv/bin/netmaster  -cluster-mode $plugin -dns-enable=false -cluster-store $cstore &> /var/contiv/log/netmaster.log
       else
           /contiv/bin/netmaster -cluster-mode $plugin -dns-enable=false  &> /var/contiv/log/netmaster.log
       fi
       echo "CRITICAL : Net Master has exited, Respawn in 5"
       sleep 5
   done &
fi
   
if [ $netplugin == true ]; then
   modprobe openvswitch
   mkdir -p /var/contiv/log/
   while [ true ]; do
       if [ "$cstore" != "" ]; then
           cstore_param="-cluster-store"
       fi
       if [ "$vtep_ip" != "" ]; then
           vtep_ip_param="-vtep-ip"
       fi
       if [ "$vlan_if" != "" ]; then
           vlan_if_param="-vlan-if"
       fi
       /contiv/bin/netplugin $cstore_param $cstore $vtep_ip_param $vtep_ip $vlan_if_param $vlan_if -plugin-mode $plugin &> /var/contiv/log/netplugin.log
       echo "CRITICAL : Net Plugin has exited, Respawn in 5"
       sleep 5
   done &
fi


while true; do sleep 1; done
