#!/usr/bin/python
import os
import sys
import json
from parse import *

class SafeDict(dict):
    'Provide a default value for missing keys'
    def __missing__(self, key):
        return 'missing'

# Parse output of vagrant ssh_config and gether necessary info
def parseSshConfig(inFd):
    confDict = SafeDict()
    ansible_node = "unknown"
    for l in inFd:
        line = l.lstrip()
        res = parse("Host {hostId}", line)
        if res is not None:
            ansible_node = res['hostId']
            confDict[ansible_node] = SafeDict()
            continue

        res = parse("HostName {ssh_host}", line)
        if res is not None:
            confDict[ansible_node]['ansible_ssh_host'] = res['ssh_host']
        

        res = parse("Port {ssh_port}", line)
        if res is not None:
            confDict[ansible_node]['ansible_ssh_port'] = res['ssh_port']
            continue

        res = parse("IdentityFile {pvt_key}", line)
        if res is not None:
            confDict[ansible_node]['ansible_ssh_private_key_file'] = res['pvt_key']
            continue

    return confDict

def validateHostInfo(hostInfo, host):
    if hostInfo[host] is 'missing':
        print "Info for {} not found".format(host) 
        sys.exit(-1)
    else:
        info = hostInfo[host]
        for attr in hostAttr:
            if info[attr] is 'missing':
                print "{} not found for host {}".format(attr, host)
                sys.exit(-1)

def writeHostLine(outFd, hostInfo, hConfig, comVars):
    host = hConfig['name']
    outFd.write(host)
    info = hostInfo[host]
    for attr in hostAttr:
        outFd.write(" {}={}".format(attr, info[attr]))

    outFd.write(" contiv_control_ip={}".format(hConfig['control-ip']))
    outFd.write(" contiv_network_if=enp0s9") # might need to change if box changes
    outFd.write(comVars)
    outFd.write("\n")

def readConfig():
    cFd = open("cluster_defs.json")
    res = json.load(cFd)
    return res

hostAttr = ["ansible_ssh_host", "ansible_ssh_port", "ansible_ssh_private_key_file"]
if __name__ == "__main__":
    # read cluster config
    clusterConf = readConfig()

    # get the ssh config info from vagrant
    res = os.system("vagrant ssh-config > ./.out.vagrant")
    if res != 0:
        print "Failed to get vagrant ssh config"
        sys.exit(-1)

    inFd = open('./.out.vagrant')
    try:
        hostInfo = parseSshConfig(inFd)
        inFd.close()
    finally:
        inFd.close()

    # Make sure all hosts are reported
    for mInfo in clusterConf['master']:
        validateHostInfo(hostInfo, mInfo['name'])
        service_ip = mInfo['control-ip']

    for nInfo in clusterConf['nodes']:
        validateHostInfo(hostInfo, nInfo['name'])

    common_vars = " ansible_ssh_user=vagrant"
    common_vars += " contiv_service_vip={}".format(service_ip)
    # add proxy if applicable
    proxy = os.environ.get('http_proxy')
    if proxy is not None:
        common_vars += " http_proxy={} https_proxy={}".format(proxy, proxy)

    outFd = open(".contiv_k8s_inventory", "w")
    outFd.write("[masters]\n")
    for mInfo in clusterConf['master']:
        writeHostLine(outFd, hostInfo, mInfo, common_vars)

    outFd.write("[etcd]\n")
    for mInfo in clusterConf['master']:
        writeHostLine(outFd, hostInfo, mInfo, common_vars)
    for nInfo in clusterConf['nodes']:
        writeHostLine(outFd, hostInfo, nInfo, common_vars)

    outFd.write("[nodes]\n")
    for nInfo in clusterConf['nodes']:
        writeHostLine(outFd, hostInfo, nInfo, common_vars)

    outFd.close()
