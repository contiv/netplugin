#!/usr/bin/python

# sanity tests
import testbedApi
import time
import sys
import objmodel
import testcases

# Parse command line args
if len(sys.argv) <= 1:
	print "Usage: " + sys.argv[0] + " <ip-addr> <ip-addr>..."
	sys.exit(1)

# Form address list
addrList = []
for addr in sys.argv[1:]:
	addrList.append(addr)

# Setup testbed
testbed = testbedApi.testbed(addrList)

time.sleep(15)

numCntr = testbed.numNodes() * 2
numIteration = 3

# Run the tests
testcases.startRemoveContainer(testbed, numCntr, numIteration)
testcases.startStopContainer(testbed, numCntr, numIteration)
testcases.testBasicPolicy(testbed, numCntr, numIteration)
testcases.testPolicyAddDeleteRule(testbed, numCntr, numIteration)

# Cleanup testbed
testbed.cleanup()

testbedApi.info("Sanity passed")
