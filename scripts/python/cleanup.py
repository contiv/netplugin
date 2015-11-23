#!/usr/bin/python

# sanity tests
import api.tnode
import time
import sys

# Parse command line args
if len(sys.argv) <= 1:
	print "Usage: " + sys.argv[0] + " <ip-addr> <ip-addr>..."
	sys.exit(1)

# Form address list
addrList = []
for addr in sys.argv[1:]:
	addrList.append(addr)

# Cleanup nodes
for addr in addrList:
	node = api.tnode.Node(addr)
	node.cleanupContainers()

for addr in addrList:
	# Cleanup all state before we can start
	node.stopNetmaster()
	node.cleanupDockerNetwork()
	node.stopNetplugin()
	node.cleanupMaster()
	node.cleanupSlave()
