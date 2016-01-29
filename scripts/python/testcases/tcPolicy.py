import api.tutils
import time
import sys
import api.objmodel

# Test basic group based policy
def testBasicPolicy(testbed, numContainer, numIter, encap="vxlan"):
	api.tutils.info("testBasicPolicy starting")

	tenant = api.objmodel.tenant('default')
	network = tenant.newNetwork('private', pktTag=1001, subnet="10.1.0.0/16", gateway="10.1.1.254", encap=encap)

	for iter in range(numIter):
		# Create policy
		policy = tenant.newPolicy('first')

		# create default deny Rule
		policy.addRule('1', direction="in", protocol="tcp", action="deny")

		# Create allow port 8000 Rule
		policy.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="allow")

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
			api.tutils.exit("Connection failed")
		if testbed.checkConnections(containers, 8001, False) != False:
			api.tutils.exit("Connection succeded while expecting it to fail")

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

		# Check for errors
		testbed.chekForNetpluginErrors()

		api.tutils.info("testBasicPolicy Iteration " + str(iter) + " passed")

	# Delete the network we created
	tenant.deleteNetwork('private')

	api.tutils.info("testBasicPolicy Test passed")


# Test adding/deleting rules from Policy
def testPolicyAddDeleteRule(testbed, numContainer, numIter, encap="vxlan"):
	api.tutils.info("testPolicyAddDeleteRule starting")

	tenant = api.objmodel.tenant('default')
	network = tenant.newNetwork('private', pktTag=1001, subnet="10.1.0.0/16", gateway="10.1.1.254", encap=encap)

	# Create policy
	policy = tenant.newPolicy('first')

	# create default deny Rule
	policy.addRule('1', direction="in", protocol="tcp", action="deny")

	# Create allow port 8000 Rule
	policy.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="allow")

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
		api.tutils.exit("Connection failed")
	if testbed.checkConnections(containers, 8001, False) != False:
		api.tutils.exit("Connection succeded while expecting it to fail")

	for iter in range(numIter):

		# Add a rule for port 8001
		policy.addRule('3', direction="in", priority=100, protocol="tcp", port=8001, action="allow")

		# now check connection passes
		if testbed.checkConnections(containers, 8001, True) != True:
			api.tutils.exit("Connection failed to port 8001")

		# Now delete the Rule
		policy.deleteRule('3')

		# Now verify connection fails
		if testbed.checkConnections(containers, 8001, False) != False:
			api.tutils.exit("Connection succeded while expecting it to fail")

		# Check for errors
		testbed.chekForNetpluginErrors()

		api.tutils.info("testPolicyAddDeleteRule Iteration " + str(iter) + " Passed")

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
	tenant.deleteNetwork('private')

	# Check for errors
	testbed.chekForNetpluginErrors()

	api.tutils.info("testPolicyAddDeleteRule Test passed")

# Test basic group based policy
def testPolicyFromEpg(testbed, numContainer, numIter, encap="vxlan"):
	api.tutils.info("testPolicyFromEpg starting")

	tenant = api.objmodel.tenant('default')
	network = tenant.newNetwork('private', pktTag=1001, subnet="10.1.0.0/16", gateway="10.1.1.254", encap=encap)

	for iter in range(numIter):
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
			policy.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="allow")
			# Create allow from 'common' epg rule
			policy.addRule('3', direction="in", priority=100, endpointGroup="common", network='private', protocol="tcp", port=8001, action="allow")
			group = network.newGroup(srvName, policies=[srvName])
			groups.append(group)

		# start containers
		containers = testbed.runContainers(numContainer, withService=True)

		# Start containers in common Epg
		cmnContainers = testbed.runContainersInService(numContainer, serviceName='common')

		# start netcat listeners
		testbed.startListeners(containers, [8000, 8001])

		# Check connection to all containers
		if testbed.checkConnections(containers, 8000, True) != True:
			api.tutils.exit("Connection failed")
		if testbed.checkConnections(containers, 8001, False) != False:
			api.tutils.exit("Connection succeded while expecting it to fail")
		if testbed.checkConnectionPair(cmnContainers, containers, 8001, True) != True:
			api.tutils.exit("Connection failed")

		# stop netcat listeners
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

		network.deleteGroup('common')

		# Check for errors
		testbed.chekForNetpluginErrors()

		api.tutils.info("testPolicyFromEpg Iteration " + str(iter) + " passed")

	# delete the network
	tenant.deleteNetwork('private')

	api.tutils.info("testPolicyFromEpg Test passed")
