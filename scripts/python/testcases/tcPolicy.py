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

# Test basic group based policy
def testPolicyFeatures(testbed, encap="vxlan"):
    api.tutils.info("testPolicyFeatures starting")

    tenant = api.objmodel.tenant('default')
    network = tenant.newNetwork('private', pktTag=1, subnet="10.1.0.0/16", gateway="10.1.1.254", encap=encap)
    dummyNet = tenant.newNetwork('dummy', pktTag=2, subnet="20.1.0.0/16", gateway="20.1.1.254", encap=encap)

    # Create policy
    pol1 = tenant.newPolicy('first')
    pol2 = tenant.newPolicy('second')

    # Add the policy to epg
    group1 = network.newGroup("srv1", policies=["first"])
    group2 = network.newGroup("srv2", policies=["second"])

    # start containers
    cnt1 = testbed.nodes[0].runContainer("ubuntu:14.04", networkName='private', serviceName='srv1')
    cnt2 = testbed.nodes[0].runContainer("ubuntu:14.04", networkName='private', serviceName='srv2')

    # start netcast listeners
    cnt1.startListener(8000)
    cnt1.startListener(8001)
    cnt2.startListener(8000)
    cnt2.startListener(8001)

    # Policy features to test
    # - from ip address
    # - from network
    # - direction out
    # - to ip address
    # - to network
    # - icmp protocol
    # - udp protocol -- ??? FIXME: netcat doesnt support udp very well.

    # Make sure connection succeds without any rule
    if cnt2.checkConnection(cnt1.getIpAddr(), 8000) != True:
        api.tutils.exit("Connection failed")

    api.tutils.log("Teasing incoming rules")

    # create incoming deny rule and verify connection fails
    pol1.addRule('1', direction="in", protocol="tcp", action="deny")
    if cnt2.checkConnection(cnt1.getIpAddr(), 8000) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")

    # Add incoming specific rule and connection succeds
    pol1.addRule('2', direction="in", priority=100, protocol="tcp", port=8000, action="allow")
    if cnt2.checkConnection(cnt1.getIpAddr(), 8000) != True:
        api.tutils.exit("Connection failed")
    if cnt2.checkConnection(cnt1.getIpAddr(), 8001) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")

    # Add incoming from-EPG rule and verify connection succeds
    pol1.addRule('3', direction="in", priority=100, endpointGroup="srv2", network='private', protocol="tcp", port=8001, action="allow")
    if cnt2.checkConnection(cnt1.getIpAddr(), 8001) != True:
        api.tutils.exit("Connection failed")

    # delete from-epg rule and verify connection fails
    pol1.deleteRule('3')
    if cnt2.checkConnection(cnt1.getIpAddr(), 8001) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")

    # Add incoming  from-network rule and verify connection succeds
    pol1.addRule('3', direction="in", priority=10, network='private', protocol="tcp", action="allow")
    pol1.addRule('4', direction="in", priority=100, network='dummy', protocol="tcp", action="deny")
    if cnt2.checkConnection(cnt1.getIpAddr(), 8001) != True:
        api.tutils.exit("Connection failed")

    # delete rule and verify connection fails
    pol1.deleteRule('3')
    pol1.deleteRule('4')
    if cnt2.checkConnection(cnt1.getIpAddr(), 8001) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")


    # Add incoming from-ipaddress rule and verify connection succeds
    pol1.addRule('3', direction="in", priority=10, ipAddress=cnt2.getIpAddr(), protocol="tcp", action="allow")
    pol1.addRule('4', direction="in", priority=100, ipAddress="20.1.1.1/24", protocol="tcp", action="deny")
    if cnt2.checkConnection(cnt1.getIpAddr(), 8001) != True:
        api.tutils.exit("Connection failed")

    # delete rules
    pol1.deleteRule('4')
    pol1.deleteRule('3')
    pol1.deleteRule('2')
    pol1.deleteRule('1')

    ############## Outgoing rule tests ######################
    api.tutils.log("Teasing outgoing rules")

    # Make sure connection succeds without any rule
    if cnt2.checkConnection(cnt1.getIpAddr(), 8000) != True:
        api.tutils.exit("Connection failed")

    # create outgoign deny rule and verify connection fails
    pol1.addRule('1', direction="out", protocol="tcp", action="deny")
    if cnt1.checkConnection(cnt2.getIpAddr(), 8000) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")

    # Add outgoing specific rule and connection succeds
    pol1.addRule('2', direction="out", priority=100, protocol="tcp", port=8000, action="allow")
    if cnt1.checkConnection(cnt2.getIpAddr(), 8000) != True:
        api.tutils.exit("Connection failed")
    if cnt1.checkConnection(cnt2.getIpAddr(), 8001) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")

    # Add outgoing to-EPG rule and verify connection succeds
    pol1.addRule('3', direction="out", priority=100, endpointGroup="srv2", network='private', protocol="tcp", port=8001, action="allow")
    if cnt1.checkConnection(cnt2.getIpAddr(), 8001) != True:
        api.tutils.exit("Connection failed")

    # delete to-epg rule and verify connection fails
    pol1.deleteRule('3')
    if cnt1.checkConnection(cnt2.getIpAddr(), 8001) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")

    # Add outgoing to-network rule and verify connection succeds
    pol1.addRule('3', direction="out", priority=10, network='private', protocol="tcp", action="allow")
    pol1.addRule('4', direction="out", priority=100, network='dummy', protocol="tcp", action="deny")
    if cnt1.checkConnection(cnt2.getIpAddr(), 8001) != True:
        api.tutils.exit("Connection failed")

    # delete rule and verify connection fails
    pol1.deleteRule('3')
    pol1.deleteRule('4')
    if cnt1.checkConnection(cnt2.getIpAddr(), 8001) != False:
        api.tutils.exit("Connection succeeded while expecting to fail")


    # Add outgoing to to-ipaddress rule and verify connection succeds
    pol1.addRule('3', direction="out", priority=10, ipAddress=cnt2.getIpAddr(), protocol="tcp", action="allow")
    pol1.addRule('4', direction="out", priority=100, ipAddress="20.1.1.1/24", protocol="tcp", action="deny")
    if cnt1.checkConnection(cnt2.getIpAddr(), 8001) != True:
        api.tutils.exit("Connection failed")

    # delete rules
    pol1.deleteRule('4')
    pol1.deleteRule('3')
    pol1.deleteRule('2')
    pol1.deleteRule('1')

    #################### ICMP rule test ################
    api.tutils.log("Teasing ICMP rules")

    # Make sure ping succeds without any rule
    cnt1.checkPing(cnt2.getIpAddr())

    # Deny incoming ICMP and make sure ping fails
    pol1.addRule('1', direction="in", protocol="icmp", action="deny")
    cnt1.checkPingFailure(cnt2.getIpAddr())

    # Add more specific rule and verify ping succeds
    pol1.addRule('2', direction="in", priority=100, ipAddress=cnt2.getIpAddr(), protocol="icmp", action="allow")
    cnt1.checkPing(cnt2.getIpAddr())

    # Delete specific rule and verify ping fails
    pol1.deleteRule('2')
    cnt1.checkPingFailure(cnt2.getIpAddr())

    # delete rule and make sure ping succeds
    pol1.deleteRule('1')
    cnt1.checkPing(cnt2.getIpAddr())

    # stop netcast listeners
    cnt1.stopListener()
    cnt2.stopListener()

    # remove containers
    cnt1.remove()
    cnt2.remove()

    # delete epg
    network.deleteGroup("srv1")
    network.deleteGroup("srv2")

    # Remove the policy and rules
    tenant.deletePolicy('first')
    tenant.deletePolicy('second')

    # Check for errors
    testbed.chekForNetpluginErrors()

    # Delete the network we created
    tenant.deleteNetwork('private')
    tenant.deleteNetwork('dummy')

    api.tutils.info("testPolicyFeatures Test passed")
