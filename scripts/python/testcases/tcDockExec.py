import api.tutils
import time
import sys
import api.objmodel

# Repeatedly execute docker exec.
# This tests docker exec to catch a bug in docker. hence its not included in sanity tests
def testDockExecRepeate(testbed, numContainer, numIteration, numExec):
    api.tutils.info("testDockExecRepeate Test starting")

    tenant = api.objmodel.tenant('default')
    network = tenant.newNetwork('private', pktTag=1001, subnet="10.1.0.0/16", gateway="10.1.1.254")

    for iter in range(numIteration):
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

        # Run exec multiple times
        for eidx in range(numExec):
            # start netcast listeners
            testbed.startListeners(containers, [8000, 8001])

            cntrIpList = []
            # Read all IP addresses
            for cnt in containers:
                cntrIp = cnt.getIpAddr()
                cntrIpList.append(cntrIp)

            # Check connection to all containers from one container on each node
            for cidx, cnt in enumerate(containers):
                for aidx, ipAddr in enumerate(cntrIpList):
                    if cidx != aidx:
                        ret = cnt.checkConnection(ipAddr, 8000)
                        # If connection status is not what we were expecting, we are done.
                        if ret != True:
                            api.tutils.exit("Connection failed")

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

        api.tutils.info("dockExec Iteration " + str(iter) + " passed")

    # Delete the network we created
    tenant.deleteNetwork('private')

    api.tutils.info("testDockExecRepeate Test passed")
