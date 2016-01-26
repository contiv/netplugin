#!/bin/bash

top_dir=$(git rev-parse --show-toplevel | sed 's|/[^/]*$||')
# run ansible
ansible-playbook -i .contiv_k8s_inventory ../../../../../contrib/ansible/cluster.yml --tags "contiv_restart" -e "networking=contiv contiv_bin_path=$top_dir/contiv_bin"
