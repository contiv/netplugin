#!/bin/bash
#Initialize complete contiv container. Start OVS and Net Plugin

/contiv/scripts/ovsInit.sh
/contiv/scripts/contivInit.sh


while true; do sleep 1; done
