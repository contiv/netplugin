#!/bin/bash
#Initialize contiv Net Plugin and Net Master as required
# Values below need to be tailored as per needs

set -x

source /contiv/scripts/contivRc

if [ $IS_NETMASTER == 1 ]; then
    /contiv/bin/netmaster 2>/var/log/contiv/netmaster.errlog 1>/var/log/contiv/netmaster.log &
fi

echo "$NETMASTER_IP  netmaster" >/etc/hosts
echo "0.0.0.0 localhost" >>/etc/hosts
export no_proxy="0.0.0.0, $NETMASTER_IP"

if [ not $CONTIV_FWD_MODE == "routing" ]; then
    CONTIV_FWD_MODE="bridge"
fi

/contiv/bin/netplugin --netmode vlan --plugin-mode docker --vtep-ip $VTEP_IP --vlan-if $VLAN_IF --fwdmode $CONTIV_FWD_MODE 2>/var/log/contiv/netplugin.errlog 1>/var/log/contiv/netplugin.log &
