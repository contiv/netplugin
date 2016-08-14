#!/usr/bin/python

# Start netplugin and netmaster
import api.tnode
import time
import sys
import os
import argparse

# Parse command line args
# Create the parser and sub parser
parser = argparse.ArgumentParser()
parser.add_argument("-nodes", required=True, help="list of nodes(comma seperated)")
parser.add_argument("-user", default='vagrant', help="User id for ssh")
parser.add_argument("-password", default='vagrant', help="password for ssh")
parser.add_argument("-binpath", default='/opt/gopath/bin', help="netplugin/netmaster binary path")

# Parse the args
args = parser.parse_args()
addrList = args.nodes.split(",")

# Use tnode class object to gather information for all nodes.
nodes = []
for addr in addrList:
    node = api.tnode.Node(addr,args.user,args.password,args.binpath)
    nodes.append(node)

gopath = "/opt/gopath"
scriptPath = gopath + "/src/github.com/contiv/netplugin/scripts"
swarmScript= scriptPath + "/start-swarm.sh"
print "SwarmScript is : " + swarmScript

print "Stopping and removing swarm containers from all Nodes"
for node in nodes:
    command = swarmScript + " stop " + node.addr + " > /tmp/swarmStop.log 2>&1"
    node.utilSwarmContainer(command)

print "Pulling and starting swarm containers from all Nodes"
for node in nodes:
    command = swarmScript + " start " + node.addr + " > /tmp/startSwarm.log 2>&1"
    node.utilSwarmContainer(command)

time.sleep(15)

print "################### Swarm cluster is up  #####################"
os._exit(0)
