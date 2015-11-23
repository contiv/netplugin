#!/bin/bash
/etc/init.d/openvswitch-switch restart
ovs-vsctl set-manager tcp:127.0.0.1:6640
ovs-vsctl set-manager ptcp:6640
while true
do
    echo service is running >> service.log
    sleep 10
done 
