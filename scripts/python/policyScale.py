#!/usr/bin/env python

# sanity tests
import api.tbed
import api.tutils
import time
import sys
import api.objmodel

# create lot of policies
def createPolicies(numPolicy, numRulesPerPolicy):
	tenant = api.objmodel.tenant('default')
	network = tenant.newNetwork('private', pktTag=10, subnet="20.1.0.0/16", gateway="20.1.1.254", encap="vlan")

	for pid in range(numPolicy):
		pname = 'policy' + str(pid + 1)
		# Create policy
		policy = tenant.newPolicy(pname)

		# create default deny Rule
		policy.addRule('1', direction="in", protocol="tcp", action="deny")

		# Create Rules
		for rid in range(numRulesPerPolicy):
			# Create allow port xxx Rule
			policy.addRule(str(2 + rid), direction="in", priority=10, protocol="tcp", port=(8000 + rid), action="allow")

		# Add the policy to epg
		epgName = "srv" + str(pid)
		group = network.newGroup(epgName, policies=[pname])

# Test connections
def testConnections(testbed, numContainer):
	# start containers
	containers = testbed.runContainers(numContainer)


	# start netcast listeners
	testbed.startListeners(containers, [8000, 7999])

	# Check connection to all containers
	if testbed.checkConnections(containers, 8000, True) != True:
		api.tutils.exit("Connection failed")
	if testbed.checkConnections(containers, 7999, False) != False:
		api.tutils.exit("Connection succeeded while expecting it to fail")

	# stop netcast listeners
	testbed.stopListeners(containers)

	# remove containers
	testbed.removeContainers(containers)

# Cleanup all policies
def cleanupPolicies(numPolicy, numRulesPerPolicy):
	tenant = api.objmodel.tenant('default')
	network = tenant.newNetwork('private', pktTag=10, subnet="20.1.0.0/16", gateway="20.1.1.254", encap="vlan")
	for pid in range(numPolicy):
		pname = 'policy' + str(pid + 1)
		policy = tenant.newPolicy(pname)

		# Remove policy from epg and delete epg
		epgName = "srv" + str(pid)
		network.deleteGroup(epgName)

		# Remove the policy and rules
		tenant.deletePolicy(pname)

# Test policy scale
def testPolicyScale(testbed, numCntr, numIter, numPolicies, numRulesPerPolicy):
	for iter in range(numIter):
		# Create policies
		createPolicies(numPolicies, numRulesPerPolicy)

		# Verify policy is working
		testConnections(testbed, numCntr)

		# Cleanup policies
		cleanupPolicies(numPolicies, numRulesPerPolicy)

		api.tutils.info("testPolicyScale Iteration " + str(iter) + " Passed")


if __name__ == "__main__":
	# Parse command line args
	if len(sys.argv) <= 3:
		print "Usage: " + sys.argv[0] + " <create|delete|test> <numPolicies> <numRulesPerPolicy> [<ip-addr> <ip-addr> ...]"
		sys.exit(1)

	command = sys.argv[1]
	numPolicies = int(sys.argv[2])
	numRulesPerPolicy = int(sys.argv[3])

	# Create policies
	if command == "create":
		createPolicies(numPolicies, numRulesPerPolicy)
	elif command == "delete":
		cleanupPolicies(numPolicies, numRulesPerPolicy)
	elif command == "test":
		# Form address list
		addrList = []
		for addr in sys.argv[4:]:
			addrList.append(addr)

		# Setup testbed
		testbed = api.tbed.Testbed(addrList)

		time.sleep(5)

		numCntr = testbed.numNodes() * 2
		numIter = 4

		# Perform the test
		testPolicyScale(testbed, numCntr, numIter, numPolicies, numRulesPerPolicy)

		# Cleanup testbed
		testbed.cleanup()

		api.tutils.info("Policy Scale test PASSES")

	else:
		print "Unknown command " + command
