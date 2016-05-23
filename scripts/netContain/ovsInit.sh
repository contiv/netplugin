#!/bin/bash
#Start OVS in the Contiv Container

sleep 2

if [ -d "/etc/openvswitch" ]; then
  if [ -f "/etc/openvswitch/conf.db" ]; then
     echo "DB Exists No Need to Create"
  else
     ovsdb-tool create  /etc/openvswitch/conf.db /usr/share/openvswitch/vswitch.ovsschema
  fi
else
  echo "Open V Switch not mounted from Host"
fi

ovsdb-server --remote=punix:/var/run/openvswitch/db.sock --remote=db:Open_vSwitch,Open_vSwitch,manager_options --private-key=db:Open_vSwitch,SSL,private_key --certificate=db:Open_vSwitch,SSL,certificate --bootstrap-ca-cert=db:Open_vSwitch,SSL,ca_cert --log-file=/var/log/contiv/ovs-db.log -vsyslog:dbg -vfile:dbg --pidfile --detach /etc/openvswitch/conf.db

ovs-vswitchd -v --pidfile --detach --log-file=/var/log/contiv/ovs-vswitchd.log -vconsole:err -vsyslog:info -vfile:info &

ovs-vsctl set-manager tcp:127.0.0.1:6640 

ovs-vsctl set-manager ptcp:6640

