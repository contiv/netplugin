#!/usr/bin/python

# sanity tests
import testbedApi
import time
import sys
import objmodel

# Start/Remove container test
def startRemoveContainer(testbed, numContainer, numIter):
	for iter in range(numIter):
		# Start the containers
		containers = testbed.runContainers(numContainer)

		# Perform ping test on the containers
		testbed.pingTest(containers)

		# remove containers
		for cnt in containers:
			cnt.remove()

		# Iteration is done
		testbedApi.info("Iteration " + str(iter) + " Passed")

	# Test is done
	testbedApi.info("startRemoveContainer Test passed")

# start/stop containers
def startStopContainer(testbed, numContainer, numIter):
	# Start the containers
	containers = testbed.runContainers(numContainer)

	# Perform ping test on the containers
	testbed.pingTest(containers)

	for iter in range(numIter):

		# Stop the containers
		for cnt in containers:
			cnt.stop()

		# Start the containers
		for cnt in containers:
			cnt.start()

		# Perform ping test on the containers
		testbed.pingTest(containers)

		# Iteration is done
		testbedApi.info("Iteration " + str(iter) + " Passed")

	# remove containers
	for cnt in containers:
		cnt.remove()

	# Test is done
	testbedApi.info("startStopContainer Test passed")

# Test basic group based policy
def testBasicPolicy(testbed, numContainer, numIter):
	for iter in range(numIter):
		tenant = objmodel.tenant('default')
		network = tenant.newNetwork('private')

		# Create policy
		policy = tenant.newPolicy('first')

		# create default deny Rule
		policy.addRule('1', direction="in", protocol="tcp", action="deny")

		# Create allow port 8000 Rule
		policy.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="accept")

		# Add the policy to epg
		groups = []
		for cntIdx in range(numContainer):
			nodeIdx = cntIdx % testbed.numNodes()
			epgName = "srv" + str(cntIdx)
			group = network.newGroup(epgName, policies=["first"])
			groups.append(group)

		# start containers
		containers = testbed.runContainers(numContainer, withService=True)

		# start netcast listeners
		testbed.startListeners(containers, [8000, 8001])

		# Check connection to all containers
		if testbed.checkConnections(containers, 8000, True) != True:
			testbedApi.exit("Connection failed")
		if testbed.checkConnections(containers, 8001, False) != False:
			testbedApi.exit("Connection succeded while expecting it to fail")

		# stop netcast listeners
		testbed.stopListeners(containers)

		# remove containers
		testbed.removeContainers(containers)

		# Remove policy from epg
		for group in groups:
			group.removePolicy("first")

		# delete epg
		for cntIdx in range(numContainer):
			nodeIdx = cntIdx % testbed.numNodes()
			epgName = "srv" + str(cntIdx)
			network.deleteGroup(epgName)

		# Remove the policy and rules
		policy.deleteRule('1')
		policy.deleteRule('2')
		tenant.deletePolicy('first')

		testbedApi.info("testBasicPolicy Iteration " + str(iter) + " passed")

	testbedApi.info("testBasicPolicy Test passed")


# Test adding/deleting rules from Policy
def testPolicyAddDeleteRule(testbed, numContainer, numIter):
	tenant = objmodel.tenant('default')
	network = tenant.newNetwork('private')

	# Create policy
	policy = tenant.newPolicy('first')

	# create default deny Rule
	policy.addRule('1', direction="in", protocol="tcp", action="deny")

	# Create allow port 8000 Rule
	policy.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="accept")

	# Add the policy to epg
	groups = []
	for cntIdx in range(numContainer):
		nodeIdx = cntIdx % testbed.numNodes()
		epgName = "srv" + str(cntIdx)
		group = network.newGroup(epgName, policies=["first"])
		groups.append(group)

	# start containers
	containers = testbed.runContainers(numContainer, withService=True)

	# start netcast listeners
	testbed.startListeners(containers, [8000, 8001])

	# Check connection to all containers
	if testbed.checkConnections(containers, 8000, True) != True:
		testbedApi.exit("Connection failed")
	if testbed.checkConnections(containers, 8001, False) != False:
		testbedApi.exit("Connection succeded while expecting it to fail")

	for iter in range(numIter):

		# Add a rule for port 8001
		policy.addRule('3', direction="in", priority=100, protocol="tcp", port=8001, action="accept")

		# now check connection passes
		if testbed.checkConnections(containers, 8000, True) != True:
			testbedApi.exit("Connection failed")
		if testbed.checkConnections(containers, 8001, True) != True:
			testbedApi.exit("Connection failed to port 8001")

		# Now delete the Rule
		policy.deleteRule('3')

		# Now verify connection fails
		if testbed.checkConnections(containers, 8000, True) != True:
			testbedApi.exit("Connection failed")
		if testbed.checkConnections(containers, 8001, False) != False:
			testbedApi.exit("Connection succeded while expecting it to fail")

		testbedApi.info("testPolicyAddDeleteRule Iteration " + str(iter) + " Passed")

	# stop netcast listeners
	testbed.stopListeners(containers)

	# remove containers
	testbed.removeContainers(containers)

	# Remove policy from epg
	for group in groups:
		group.removePolicy("first")

	# delete epg
	for cntIdx in range(numContainer):
		nodeIdx = cntIdx % testbed.numNodes()
		epgName = "srv" + str(cntIdx)
		network.deleteGroup(epgName)

	# Remove the policy and rules
	policy.deleteRule('1')
	policy.deleteRule('2')
	tenant.deletePolicy('first')

	testbedApi.info("testPolicyAddDeleteRule Test passed")

# Test basic group based policy
def testPolicyFromEpg(testbed, numContainer, numIter):
	for iter in range(numIter):
		tenant = objmodel.tenant('default')
		network = tenant.newNetwork('private')
		# Create common epg
		network.newGroup('common')

		# Add the policy to epg
		groups = []
		for cntIdx in range(numContainer):
			nodeIdx = cntIdx % testbed.numNodes()
			srvName = "srv" + str(cntIdx)

			# Create policy for each service
			policy = tenant.newPolicy(srvName)

			# create default deny Rule
			policy.addRule('1', direction="in", protocol="tcp", action="deny")

			# Create allow port 8000 Rule
			policy.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="accept")
			# Create allow from 'common' epg rule
			policy.addRule('3', direction="in", priority=100, endpointGroup="common", network='private', protocol="tcp", port=8001, action="accept")
			group = network.newGroup(srvName, policies=[srvName])
			groups.append(group)

		# start containers
		containers = testbed.runContainers(numContainer, withService=True)

		# Start containers in common Epg
		cmnContainers = testbed.runContainersInService(numContainer, serviceName='common')

		# start netcast listeners
		testbed.startListeners(containers, [8000, 8001])

		# Check connection to all containers
		if testbed.checkConnections(containers, 8000, True) != True:
			testbedApi.exit("Connection failed")
		if testbed.checkConnections(containers, 8001, False) != False:
			testbedApi.exit("Connection succeded while expecting it to fail")
		if testbed.checkConnectionPair(cmnContainers, containers, 8001, True) != True:
			testbedApi.exit("Connection failed")

		# stop netcast listeners
		testbed.stopListeners(containers)

		# remove containers
		testbed.removeContainers(containers)
		testbed.removeContainers(cmnContainers)

		# delete epg
		for cntIdx in range(numContainer):
			nodeIdx = cntIdx % testbed.numNodes()
			srvName = "srv" + str(cntIdx)
			network.deleteGroup(srvName)
			tenant.deletePolicy(srvName)


		testbedApi.info("testPolicyFromEpg Iteration " + str(iter) + " passed")

	testbedApi.info("testPolicyFromEpg Test passed")
