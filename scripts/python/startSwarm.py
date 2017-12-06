#!/usr/bin/env python

# Start netplugin and netmaster
import api.tnode
import argparse
import os
import re
import time

# Parse command line args
# Create the parser and sub parser
parser = argparse.ArgumentParser()
parser.add_argument("-nodes", required=True, help="list of nodes(comma separated)")
parser.add_argument("-user", default='vagrant', help="User id for ssh")
parser.add_argument("-password", default='vagrant', help="password for ssh")
parser.add_argument("-binpath", default='/opt/gopath/bin', help="netplugin/netmaster binary path")
parser.add_argument("-swarm", default='classic_mode', help="classic_mode or swarm_mode")

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

if args.swarm == "swarm_mode":
    # Nodes leave the swarm
    for node in nodes:
        node.runCmd("docker swarm leave --force")

    # Create a swarm with Node0 as manager
    nodes[0].runCmd("docker swarm init --advertise-addr " + nodes[0].addr + ":2377")
    # Get the token for joining swarm
    out, x, y = nodes[0].runCmd("docker swarm join-token worker -q")
    token = out[0][:-1] #remove newline
    # Make all workers join the swarm
    for node in nodes[1:]:
        command = "docker swarm join --token "+ token + " " + nodes[0].addr + ":2377"
        node.runCmdThread(command)

    time.sleep(15)

    print "Check netplugin is installed and enabled"
    out, _, _ = nodes[0].runCmd("docker plugin ls")

    installed = re.search('contiv/v2plugin', out[1])

    if installed == None:
        print "Make target failed: Contiv plugin is not installed"
        os._exit(1)
        
    enabled = re.search('false', out[1])
    if enabled != None:
        print "Make target failed: Contiv plugin is installed but disabled"
        os._exit(1)

    print "################### Swarm Mode is up  #####################"
else:
    swarmScript= scriptPath + "/start-swarm.sh"
    print "SwarmScript is : " + swarmScript

    print "Stopping and removing swarm containers from all Nodes"
    for node in nodes:
        command = swarmScript + " stop " + node.addr + " > /tmp/swarmStop.log 2>&1"
        node.runCmdThread(command)

    print "Pulling and starting swarm containers from all Nodes"
    for node in nodes:
        command = swarmScript + " start " + node.addr + " > /tmp/startSwarm.log 2>&1"
        node.runCmdThread(command)
    time.sleep(15)
    print "################### Classic Swarm cluster is up  #####################"


os._exit(0)
