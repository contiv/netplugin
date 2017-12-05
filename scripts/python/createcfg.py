#!/usr/bin/env python
# script to create the config JSON input for the system tests
import os
import json
import argparse

parser = argparse.ArgumentParser()
parser.add_argument("-scheduler", default='docker', help="Scheduler used - docker or k8s")
# Install mode for k8s - Legacy vs Kubeadm.
parser.add_argument("-install_mode", default='legacy', help="Install mode - legacy or kubeadm")
parser.add_argument("-swarm_var", default='', help="Swarm host variable")
parser.add_argument("-platform", default='vagrant', help="Vagrant/baremetal")
parser.add_argument("-product", default='netplugin', help="netplugin/volplugin")
parser.add_argument("-contiv_l3", default='', help="Running in L3 mode")
parser.add_argument("-contiv_k8s", default=0, help="Running in K8s mode")
parser.add_argument("-key_file", default="/home/admin/.ssh/id_rsa", help="file path of key_file")
parser.add_argument("-binpath", default="/opt/gopath/bin", help="GOBIN path")
parser.add_argument("-hostips", default="192.168.2.10,192.168.2.11,192.168.2.12", help="Host IPs in the system")
parser.add_argument("-hostnames", default="admin,admin,admin", help="Usernames on the hosts")

parser.add_argument("-aci_mode", default='off', help="Running in ACI mode")
parser.add_argument("-short", default=False, help="Shorter version of system tests")
parser.add_argument("-containers", default=3, help="Number of containers for each test")
parser.add_argument("-iterations", default=3, help="Number of iterations for each test")
parser.add_argument("-enableDNS", default=False, help="Enabling DNS")
parser.add_argument("-contiv_cluster_store_driver", default="etcd", help="cluster store driver [etcd, consul]")
parser.add_argument("-contiv_cluster_store_urls", default="http://localhost:2379", help="cluster store urls")
parser.add_argument("-k8s_contiv_cluster_store_driver", default="etcd", help="k8s cluster store driver")
parser.add_argument("-k8s_contiv_cluster_store_urls", default="http://netmaster:6666", help="k8s cluster store urls")
parser.add_argument("-datainterfaces", default="eth2,eth3", help="Data interface")
parser.add_argument("-l3_datainterfaces", default="eth2", help="Data interface")
parser.add_argument("-k8s_datainterfaces", default="eth2", help="Data interface")
parser.add_argument("-mgmtinterface", default="eth1", help="Control interface")
parser.add_argument("-vlan", default="1120-1150", help="vlan range")
parser.add_argument("-vxlan", default="1-10000", help="vxlan range")
parser.add_argument("-subnet", default="10.1.1.0/24", help="subnet for ACI testing")
parser.add_argument("-gateway", default="10.1.1.254", help="gateway for ACI testing")
parser.add_argument("-network", default="TestNet", help="network name for ACI testing")
parser.add_argument("-tenant", default="TestTenant", help="tenant name for ACI testing")
parser.add_argument("-encap", default="vlan", help="encapsulation for ACI testing")

args = parser.parse_args()
data = {}
data['scheduler'] = args.scheduler
data['install_mode'] = args.install_mode
data['swarm_variable'] = args.swarm_var
data['platform'] = args.platform
data['product'] = args.product
data['aci_mode'] = args.aci_mode
data['short'] = args.short
data['containers'] = args.containers
data['iterations'] = args.iterations
data['enableDNS'] = args.enableDNS
data['contiv_l3'] = args.contiv_l3
if args.scheduler == 'k8s':
    data['contiv_cluster_store_driver'] = args.k8s_contiv_cluster_store_driver
    data['contiv_cluster_store_urls'] = args.k8s_contiv_cluster_store_urls
else:
    data['contiv_cluster_store_driver'] = args.contiv_cluster_store_driver
    data['contiv_cluster_store_urls'] = args.contiv_cluster_store_urls
data['keyFile'] = args.key_file
data['binpath'] = args.binpath
data['hostips'] = args.hostips
data['hostusernames'] = args.hostnames
if args.contiv_l3 == '':
    data['dataInterfaces'] = args.datainterfaces
else:
    data['dataInterfaces'] = args.l3_datainterfaces
if args.scheduler == 'k8s':
    data['dataInterfaces'] = args.k8s_datainterfaces
data['install_mode'] = args.install_mode
data['mgmtInterface'] = args.mgmtinterface
data['vlan'] = args.vlan
data['vxlan'] = args.vxlan
data['subnet'] = args.subnet
data['gateway'] = args.gateway
data['network'] = args.network
data['tenant'] = args.tenant
data['encap'] = args.encap

filepath = os.environ['GOPATH'] + '/src/github.com/contiv/netplugin/test/systemtests/cfg.json'
with open(filepath, 'w') as outfile:
	print("Generating the config file: " + filepath)
	json.dump(data, outfile)

os._exit(0)
