import api.tutils
import time
import sys
import api.objmodel

# Test configure ACI mode
def testACIMode(testbed):
    api.tutils.info("testACIMode starting")
    api.objmodel.setFabricMode("aci")

    # Create a network
    testTen = api.objmodel.tenant('default')
    testNet = testTen.newNetwork("aciNet", 0, "22.2.2.0/24", "22.2.2.254", "vlan")

    # Create two epgs
    epgA = testNet.newGroup("epgA")
    epgB = testNet.newGroup("epgB")

    # Start two containers each on epgA and epgB
    cA1 = testbed.runContainerOnNode(0, "epgA.aciNet")
    cA2 = testbed.runContainerOnNode(0, "epgA.aciNet")
    cB1 = testbed.runContainerOnNode(0, "epgB.aciNet")
    cB2 = testbed.runContainerOnNode(0, "epgB.aciNet")

    # Verify cA1 can ping cA2
    cA1.checkPing(cA2.getIpAddr()) 

    # Verify cB1 can ping cB2
    cB1.checkPing(cB2.getIpAddr()) 

    # Verify cA1 cannot ping cB1
    cA1.checkPingFailure(cB1.getIpAddr()) 

    # remove containers
    testbed.removeContainers([cA1, cA2, cB1, cB2])

    # delete epgs
    testNet.deleteGroup("epgA")
    testNet.deleteGroup("epgB")

    # delete network
    testTen.deleteNetwork("aciNet")
    api.objmodel.setFabricMode("default")

    # Check for errors
    testbed.checkForNetpluginErrors()

    api.tutils.info("testACIMode Test passed")
