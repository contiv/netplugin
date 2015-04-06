#!/bin/bash
/etc/init.d/openvswitch-switch restart
while true
do
    echo service is running >> service.log
    sleep 10
done 
