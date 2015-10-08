#!/usr/bin/python

# Start netplugin and netmaster
import testbedApi
import time
import sys
import os
import setupProxy

# Parse command line args
if len(sys.argv) <= 1:
	print "Usage: " + sys.argv[0] + " <ip-addr> <ip-addr>..."
	print "This starts netplugin and netmaster"
	sys.exit(1)

# Form address list
addrList = []
for addr in sys.argv[1:]:
	addrList.append(addr)

# Cleanup all state and start netplugin/netmaster
testbed = testbedApi.testbed(addrList)
time.sleep(2)

# Setup proxy
setupProxy.setupProxy()

os._exit(0)
