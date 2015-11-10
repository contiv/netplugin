#!/usr/bin/python

# sanity tests
import testbedApi
import time
import sys
import objmodel

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
	node = testbedApi.vagrantNode(addr)
	node.cleanupContainers()
	
for addr in addrList:
	# Cleanup all state before we can start
	node.stopNetmaster()
	node.stopNetplugin()
	node.cleanupMaster()
	node.cleanupSlave()
