import api.tutils
import time
import sys
import api.objmodel

# Test network add delete
# This test expects atleast 4 containers per node
def testAddDeleteNetwork(testbed, numContainer, numIter, encap="vxlan"):
    api.tutils.info("testAddDeleteNetwork starting")

    tenant = api.objmodel.tenant('default')

    for iter in range(numIter):
        # create networks
        networks = []
        netNames = []

        # Determine the number of networks so that each node has atleast two containers in each network.
        # so that we can test both single host and multi-host scenarios
        if numContainer >= (testbed.numNodes() * 2):
            numNetworks = numContainer / (testbed.numNodes() * 2)
        else:
            numNetworks = 1

        for idx in range(numNetworks):
            netName = "net" + str(idx)
            subnet = "10.1." + str(idx) + ".0/24"
            gateway = "10.1." + str(idx) + ".254"
            network = tenant.newNetwork(netName, pktTag=(1001 + idx), subnet=subnet, gateway=gateway, encap=encap)
            networks.append(network)
            netNames.append(netName)

        # start containers
        containers = testbed.runContainersInNetworks(numContainer, netNames)

        cntrIpList = []
        # Read all IP addresses
        for cnt in containers:
            cntrIp = cnt.getIpAddr()
            cntrIpList.append(cntrIp)

        # Test ping from each container to other container in same network
        # We expect ping success for each container in same network
        #    AND    ping failure for each container in different network
        for cidx, cnt in enumerate(containers):
            if (cidx % (testbed.numNodes() * numNetworks)) == 0:
                for ipidx, ipAddr in enumerate(cntrIpList):
                    if (ipidx % len(networks)) == (cidx % len(networks)):
                        cnt.checkPing(ipAddr)
                    elif ((cidx + 1) % len(networks)) == (ipidx  % len(networks)):
                        cnt.checkPingFailure(ipAddr)

        # remove containers
        testbed.removeContainers(containers)

        # delete networks
        for idx in range(numContainer/2):
            netName = "net" + str(idx)
            tenant.deleteNetwork(netName)

        # Check for errors
        testbed.chekForNetpluginErrors()

        api.tutils.info("testAddDeleteNetwork Iteration " + str(iter) + " passed")

    api.tutils.info("testAddDeleteNetwork Test passed")

# Test tenant add delete
def testAddDeleteTenant(testbed, numContainer, numIter, encap="vxlan"):
    api.tutils.info("testAddDeleteTenant starting")

    for iter in range(numIter):
        # create tenants and networks
        tenants = []
        networks = []
        netNames = []
        for idx in range(numContainer/2):
            nodeIdx = idx % testbed.numNodes()
            tenantName = "tenant" + str(idx)
            tenant = api.objmodel.tenant(tenantName)
            tenants.append(tenant)
            netName = "net" + str(idx)
            subnet = "10.1." + str(idx) + ".0/24"
            gateway = "10.1." + str(idx) + ".254"
            network = tenant.newNetwork(netName, pktTag=(1001 + idx), subnet=subnet, gateway=gateway, encap=encap)
            networks.append(network)
            netNames.append(netName + "/" + tenantName)


        # start containers
        containers = testbed.runContainersInNetworks(numContainer, netNames)

        cntrIpList = []
        # Read all IP addresses
        for cnt in containers:
            cntrIp = cnt.getIpAddr()
            cntrIpList.append(cntrIp)

        # Test ping from each container to other container in same network
        for cidx, cnt in enumerate(containers):
            if (cidx % testbed.numNodes()) == 0:
                for ipidx, ipAddr in enumerate(cntrIpList):
                    if (ipidx % len(networks)) == (cidx % len(networks)):
                        cnt.checkPing(ipAddr)

        # remove containers
        testbed.removeContainers(containers)

        # delete networks and tenants
        for idx, tenant in enumerate(tenants):
            netName = "net" + str(idx)
            tenant.deleteNetwork(netName)
            tenant.delete()

        # Check for errors
        testbed.chekForNetpluginErrors()

        api.tutils.info("testAddDeleteTenant Iteration " + str(iter) + " passed")

    api.tutils.info("testAddDeleteTenant Test passed")
