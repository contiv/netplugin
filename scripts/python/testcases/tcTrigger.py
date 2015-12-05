import api.tutils
import time
import sys
import api.objmodel
import random

# Check ping between all containers in the same network
def checkPingContainersInNetworks(containers, networks):
    cntrIpList = []
    # Read all IP addresses
    for cnt in containers:
        cntrIp = cnt.getIpAddr()
        cntrIpList.append(cntrIp)

    nidx = random.randint(0, (len(networks) - 1))

    # Test ping from each container to other container in same network
    # We expect ping success for each container in same network
    for cidx, cnt in enumerate(containers):
        for ipidx, ipAddr in enumerate(cntrIpList):
            if (ipidx % len(networks)) == (cidx % len(networks)) and (cidx % len(networks) == nidx):
                cnt.checkPing(ipAddr)

# Check full mesh connection to all containers within a group
def checkConnectionsWithinGroup(containers, groups, port, success):
    cntrIpList = []
    # Read all IP addresses
    for cnt in containers:
        cntrIp = cnt.getIpAddr()
        cntrIpList.append(cntrIp)

    gidx = random.randint(0, (len(groups) - 1))

    # Check connection to all containers
    for cidx, cnt in enumerate(containers):
        for aidx, ipAddr in enumerate(cntrIpList):
            if (cidx % len(groups)) == (aidx % len(groups)) and (cidx % len(groups) == gidx):
                ret = cnt.checkConnection(ipAddr, port)
                # If connection status is not what we were expecting, we are done.
                if ret != success:
                    return ret

    # Return
    return success

# Check connection to all containers between two neighboring groups
def checkConnectionsAcrossGroup(containers, groups, port, success):
    cntrIpList = []
    # Read all IP addresses
    for cnt in containers:
        cntrIp = cnt.getIpAddr()
        cntrIpList.append(cntrIp)

    gidx = random.randint(0, (len(groups) - 1))

    # Check connection to all containers
    for cidx, cnt in enumerate(containers):
        for aidx, ipAddr in enumerate(cntrIpList):
            cgrp = groups[cidx % len(groups)]
            agrp = groups[aidx % len(groups)]
            if ((cidx + 1) % len(groups)) == (aidx % len(groups)) and cgrp.networkName == agrp.networkName and (cidx % len(groups) == gidx):
                ret = cnt.checkConnection(ipAddr, port)
                # If connection status is not what we were expecting, we are done.
                if ret != success:
                    return ret

    # Return
    return success

# Checks all datapth connections
def checkAllConnection(testbed, netContainers, networks, grpContainers, groups):
    # Check ping between containes in same network
    checkPingContainersInNetworks(netContainers, networks)

    # Check port 8000 and 8001 connecting succeedes within group
    if checkConnectionsWithinGroup(grpContainers, groups, 8000, True) != True:
        api.tutils.exit("Connection failed")
    if checkConnectionsWithinGroup(grpContainers, groups, 8001, True) != True:
        api.tutils.exit("Connection failed")
    if checkConnectionsAcrossGroup(grpContainers, groups, 8000, True) != True:
        api.tutils.exit("Connection failed")
    if checkConnectionsAcrossGroup(grpContainers, groups, 8001, False) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")

    # Check for errors
    testbed.chekForNetpluginErrors()

# remove all containers
def removeAllContainers(netContainers, grpContainers):
    for cnt in netContainers:
        cnt.remove()
    for cnt in grpContainers:
        cnt.remove()

# Start all containers
def startAllContainers(testbed, netNames, groupNames):
    # start containers in each network
    numCntr = len(netNames) * testbed.numNodes() * 2
    netContainers = testbed.runContainersInNetworks(numCntr, netNames)

    # start containers in each group
    numCntr = len(groupNames) * testbed.numNodes() * 2
    grpContainers = testbed.runContainersInGroups(numCntr, groupNames)

    # start netcat listeners on epg containers
    testbed.startListeners(grpContainers, [8000, 8001])

    # Return newly created containers
    return netContainers, grpContainers

# Trigger netplugin restart
def triggerNetpluginRestart(testbed):
    for node in testbed.nodes:
        api.tutils.info("Restarting netplugin on " + node.hostname)
        node.stopNetplugin()
        time.sleep(1)

        # Move old log file
        currTime = time.strftime("%H:%M:%S", time.localtime())
        node.runCmd("mv /tmp/netplugin.log /tmp/netplugin-" + currTime + ".log")

        node.startNetplugin()

        # Wait a little
        time.sleep(5)


# Trigger netmaster restart
def triggerNetmasterRestart(testbed):
    api.tutils.info("Restarting netmaster on " + testbed.nodes[0].hostname)
    testbed.nodes[0].stopNetmaster()
    time.sleep(1)

    currTime = time.strftime("%H:%M:%S", time.localtime())
    testbed.nodes[0].runCmd("mv /tmp/netmaster.log /tmp/netmaster-" + currTime + ".log")

    testbed.nodes[0].startNetmaster()

    # Wait a little
    time.sleep(10)

# Trigger removal/add of all containers
def triggerRestartContainers(testbed, netContainers, grpContainers, netNames, groupNames):
    # Remove all containers
    removeAllContainers(netContainers, grpContainers)

    # Start all containers
    return startAllContainers(testbed, netNames, groupNames)

# Tests multiple triggers and verifies datapath after each trigger
def testMultiTrigger(testbed, numIter, numTenants=1, numNetworksPerTenant=1, numGroupsPerNetwork=2):
    # create tenants and networks
    tenants = []
    networks = []
    netNames = []
    groups = []
    groupNames = []
    policies = []

    # Setup tenants, networks and policies
    for tidx in range(numTenants):
        tenantName = "tenant" + str(tidx)
        tenant = api.objmodel.tenant(tenantName)
        tenants.append(tenant)

        # Create multiple networks per tenant
        for nidx in range(numNetworksPerTenant):
            netName = "net" + str(nidx)
            subnet = "10." + str(tidx) + "." + str(nidx) + ".0/24"
            gateway = "10." + str(tidx) + "." + str(nidx) + ".254"
            pktTag = 1001 + (tidx * numNetworksPerTenant) + nidx
            network = tenant.newNetwork(netName, pktTag=pktTag, subnet=subnet, gateway=gateway, encap="vxlan")
            networks.append(network)
            netNames.append(netName + "/" + tenantName)

            # Create multiple EPGs and associated policies
            for pidx in range(numGroupsPerNetwork):
                srvName = "srv" + str(pidx)
                policyName = "srv" + str(pidx) + "_" + netName

                # Create policy for each service
                policy = tenant.newPolicy(policyName)
                policies.append(policy)

                # create default deny Rule
                policy.addRule('1', direction="in", protocol="tcp", action="deny")

                # Create allow port 8000 Rule
                policy.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="accept")

                # create the EPG
                group = network.newGroup(srvName, policies=[policyName])
                groups.append(group)
                groupNames.append(srvName + "." + netName + "/" + tenantName)

                # Create allow from this epg rule
                policy.addRule('3', direction="in", priority=100, endpointGroup=srvName, network=netName, protocol="tcp", port=8001, action="accept")


    # start containers in each network and each group
    netContainers, grpContainers = startAllContainers(testbed, netNames, groupNames)

    # Check all datapaths
    checkAllConnection(testbed, netContainers, networks, grpContainers, groups)

    # Run a random trigger and verify all connections
    for iter in range(numIter):
        triggerIdx = random.randint(1, 3)

        # Run the specific trigger
        if triggerIdx == 1:
            triggerName = "triggerNetpluginRestart"
            api.tutils.log("Performing trigger " + triggerName)
            # restart netplugin
            triggerNetpluginRestart(testbed)
        elif triggerIdx == 2:
            triggerName = "triggerNetmasterRestart"
            api.tutils.log("Performing trigger " + triggerName)
            # restart netmaster
            triggerNetmasterRestart(testbed)
        elif triggerIdx == 3:
            triggerName = "triggerRestartContainers"
            api.tutils.log("Performing trigger " + triggerName)
            # restart containers
            netContainers, grpContainers = triggerRestartContainers(testbed, netContainers, grpContainers, netNames, groupNames)
        else:
            api.tutils.exit("Unexpected value")

        api.tutils.log("Performed trigger " + triggerName + " verifying datapath")

        # Check all datapaths after each trigger
        checkAllConnection(testbed, netContainers, networks, grpContainers, groups)

        api.tutils.info("testMultiTrigger Iteration " + str(iter) + " trigger " + triggerName + " Passed")

    # Cleanup all containers
    removeAllContainers(netContainers, grpContainers)

    # Cleanup all state
    for group in groups:
        group.delete()
    for policy in policies:
        policy.delete()
    for net in networks:
        net.delete()
    for tenant in tenants:
        tenant.delete()

    # Check for cleanup errors
    testbed.chekForNetpluginErrors()

    # Done
    api.tutils.info("testMultiTrigger PASSED")
