import api.tutils
import time
import sys
import api.objmodel

# Start/Remove container test
def startRemoveContainer(testbed, numContainer, numIter, encap="vxlan"):
	api.tutils.info("startRemoveContainer starting")

	tenant = api.objmodel.tenant('default')
	network = tenant.newNetwork('private', pktTag=1001, subnet="10.1.0.0/16", gateway="10.1.1.254", encap=encap)

	for iter in range(numIter):
		# Start the containers
		containers = testbed.runContainers(numContainer)

		# Perform ping test on the containers
		testbed.pingTest(containers)

		# remove containers
		for cnt in containers:
			cnt.remove()

		# Check for errors
		testbed.chekForNetpluginErrors()

		# Iteration is done
		api.tutils.info("Iteration " + str(iter) + " Passed")

	# Delete the network we created
	tenant.deleteNetwork('private')

	# Check for errors
	testbed.chekForNetpluginErrors()

	# Test is done
	api.tutils.info("startRemoveContainer Test passed")

# start/stop containers
def startStopContainer(testbed, numContainer, numIter, encap="vxlan"):
	api.tutils.info("startStopContainer starting")

	tenant = api.objmodel.tenant('default')
	network = tenant.newNetwork('private', pktTag=1001, subnet="10.1.0.0/16", gateway="10.1.1.254", encap=encap)

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

		# Check for errors
		testbed.chekForNetpluginErrors()

		# Iteration is done
		api.tutils.info("startStopContainer iteration " + str(iter) + " Passed")

	# remove containers
	for cnt in containers:
		cnt.remove()

	# Delete the network we created
	tenant.deleteNetwork('private')

	# Check for errors
	testbed.chekForNetpluginErrors()

	# Test is done
	api.tutils.info("startStopContainer Test passed")
