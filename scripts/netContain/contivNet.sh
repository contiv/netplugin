#!/bin/bash
#Initialize complete contiv container. Start OVS and Net Plugin

ARG1=${1:-none}

if [ $ARG1 == "reinit" ]; then
    ovs-vsctl del-br contivVlanBridge
    ovs-vsctl del-br contivVxlanBridge
fi

/contiv/scripts/ovsInit.sh
/contiv/scripts/contivInit.sh


while true; do sleep 1; done
