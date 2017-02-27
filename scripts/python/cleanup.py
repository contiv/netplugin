#!/usr/bin/env python

# sanity tests
import api.tnode
import time
import sys
import argparse

# Parse command line args
# Create the parser and sub parser
parser = argparse.ArgumentParser()
parser.add_argument('--version', action='version', version='1.0.0')
parser.add_argument("-nodes", required=True, help="list of nodes(comma separated)")
parser.add_argument("-user", default='vagrant', help="User id for ssh")
parser.add_argument("-password", default='vagrant', help="password for ssh")

# Parse the args
args = parser.parse_args()
addrList = args.nodes.split(",")

nodes = []

# Cleanup nodes
for addr in addrList:
	node = api.tnode.Node(addr, args.user, args.password)
	node.cleanupContainers()
	nodes.append(node)

for node in nodes:
	# Cleanup all state before we can start
	node.stopNetmaster()
	node.cleanupDockerNetwork()
	node.stopNetplugin()
	node.stopV2Plugin()
	node.cleanupMaster()
	node.cleanupSlave()
