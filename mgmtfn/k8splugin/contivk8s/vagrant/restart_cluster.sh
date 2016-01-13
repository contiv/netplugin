#!/bin/bash

# run ansible
ansible-playbook -i .contiv_k8s_inventory ../../../../../contrib/ansible/cluster.yml --tags "contiv_restart" -e "networking=contiv"
