import tnode
import time
import sys
import tutils

# This class represents a testbed i.e, collection of nodes
class Testbed:
    # Initialize a testbed
    def __init__(self, addrList, username='vagrant', password='vagrant', binpath='/opt/gopath/bin', plugintype='binary'):
        self.nodes = []
        self.failOnError = True
        # Basic error checking
        if len(addrList) < 1:
            print "Empty address list"
            sys.exit(1)

        # Create nodes
        for addr in addrList:
            node = tnode.Node(addr, username, password, binpath)
            self.nodes.append(node)

        # Cleanup all state before we can start
        for node in self.nodes:
            node.cleanupContainers()
            node.stopNetmaster()
            node.cleanupDockerNetwork()
            node.stopNetplugin()
            node.stopV2Plugin()

        # cleanup master and slave state
        for node in self.nodes:
            node.cleanupMaster()
            node.cleanupSlave()
        # start the plugins
        if plugintype == 'v2plugin':
            self.startV2Plugin()
        else:
            self.startPluginBinaries()

    # Start legacy plugin
    def startPluginBinaries(self):
        # Start netplugin on all nodes
        for node in self.nodes:
            print "Starting netplugin on " + node.hostname
            node.startNetplugin()

        # Wait few seconds before starting netmaster
        time.sleep(3)

        # Start netmaster in the end
        for nidx, node in enumerate(self.nodes):
            # Start netmaster only on first three nodes
            if nidx < 3:
                print "Starting netmaster on " + node.hostname
                node.startNetmaster()

    # Start legacy plugin
    def startV2Plugin(self):
        # Start netplugin on all nodes
        for nidx,node in enumerate(self.nodes):
            print "Creating v2plugin on " + node.hostname
            node.createV2Plugin()
            print "Enabling v2plugin on " + node.hostname
            # first node is the swarm master
            if nidx == 0:
                node.enableV2Plugin()
            else:
                node.enableV2Plugin("worker")

    # Cleanup a testbed once test is done
    def cleanup(self):
        # Cleanup each node
        for node in self.nodes:
            print "Stopping containers on " + node.addr
            node.cleanupContainers()

        # Stop netmaster and remove networks
        for node in self.nodes:
            node.stopNetmaster()
            node.cleanupDockerNetwork()

        for node in self.nodes:
            print "Cleaning up node " + node.addr
            node.stopNetplugin()
            node.stopV2Plugin()
            node.cleanupSlave()

        # cleanup master
        print "Cleaning up master"
        self.nodes[0].cleanupMaster()


    # Number of nodes in the testbed
    def numNodes(self):
        return len(self.nodes)

    # Start containers on the testbed
    def runContainers(self, numContainer, withService=False):
        containers = []
        # Start the containers
        for cntIdx in range(numContainer):
            nodeIdx = cntIdx % self.numNodes()
            srvName = "srv" + str(cntIdx)
            if withService:
                cnt = self.nodes[nodeIdx].runContainer("ubuntu:14.04", networkName="private", serviceName=srvName, cntName=srvName)
            else:
                cnt = self.nodes[nodeIdx].runContainer("ubuntu:14.04", networkName="private", cntName=srvName)

            containers.append(cnt)

        return containers

    # Start containers in a specific service
    def runContainersInService(self, numContainer, serviceName, networkName="private"):
        containers = []
        # Start the containers
        for cntIdx in range(numContainer):
            nodeIdx = cntIdx % self.numNodes()
            cnt = self.nodes[nodeIdx].runContainer("ubuntu:14.04", networkName=networkName, serviceName=serviceName)
            containers.append(cnt)

        return containers

    # Start containers in list of networks
    def runContainersInNetworks(self, numContainer, networks):
        containers = []
        # Start the containers
        for cntIdx in range(numContainer):
            nodeIdx = cntIdx % self.numNodes()
            netIdx = cntIdx % len(networks)
            cnt = self.nodes[nodeIdx].runContainer("ubuntu:14.04", networkName=networks[netIdx])
            containers.append(cnt)

        return containers

    # Start containers in list of groups
    def runContainersInGroups(self, numContainer, groups):
        containers = []
        # Start the containers
        for cntIdx in range(numContainer):
            nodeIdx = cntIdx % self.numNodes()
            gidx = cntIdx % len(groups)
            netName = groups[gidx].split(".")[1]
            svcName = groups[gidx].split('.')[0]
            cnt = self.nodes[nodeIdx].runContainer("ubuntu:14.04", networkName=netName, serviceName=svcName)
            containers.append(cnt)

        return containers

    # Start a container on the specified node
    def runContainerOnNode(self, nodeIdx, group):
        netName = group.split(".")[1]
        svcName = group.split('.')[0]
        cnt = self.nodes[nodeIdx].runContainer("ubuntu:14.04", networkName=netName, serviceName=svcName)
        return cnt

    # Remove containers
    def removeContainers(self, containers):
        # remove containers
        for cnt in containers:
            cnt.remove()

    # start all netcast listeners
    def startListeners(self, containers, ports):
        # start netcast listeners
        for cnt in containers:
            for port in ports:
                cnt.startListener(port)

    def stopListeners(self, containers):
        # stop netcast listeners
        for cnt in containers:
            cnt.stopListener()

    def pingTest(self, containers):
        cntrIpList = []
        # Read all IP addresses
        for cnt in containers:
            cntrIp = cnt.getIpAddr()
            cntrIpList.append(cntrIp)

        # Test ping to all other containers from one container in each node
        for cidx, cnt in enumerate(containers):
            if (cidx < self.numNodes()):
                for ipAddr in cntrIpList:
                    cnt.checkPing(ipAddr)

        return True

    # Check full mesh connection to all containers
    def checkConnections(self, containers, port, success):
        cntrIpList = []
        # Read all IP addresses
        for cnt in containers:
            cntrIp = cnt.getIpAddr()
            cntrIpList.append(cntrIp)

        # Check connection to all containers from one container on each node
        for cidx, cnt in enumerate(containers):
            if (cidx < self.numNodes()):
                for aidx, ipAddr in enumerate(cntrIpList):
                    if cidx != aidx:
                        ret = cnt.checkConnection(ipAddr, port)
                        # If connection status is not what we were expecting, we are done.
                        if ret != success:
                            return ret

        # Return
        return success

    # Check bipartite connection between two list of containers
    def checkConnectionPair(self, fromContainers, toContainers, port, success):
        toIpList = []
        # Read all IP addresses
        for cnt in toContainers:
            cntrIp = cnt.getIpAddr()
            toIpList.append(cntrIp)

        # Check connection from each container
        for cidx, cnt in enumerate(fromContainers):
            for aidx, ipAddr in enumerate(toIpList):
                if cnt.getIpAddr() != ipAddr:
                    ret = cnt.checkConnection(ipAddr, port)
                    # If connection status is not what we were expecting, we are done.
                    if ret != success:
                        return ret

        # Return
        return success

    # Look for any error logs on all nodes
    def checkForNetpluginErrors(self):
        for node in self.nodes:
            ret = node.checkForNetpluginErrors()
            if ret == False and self.failOnError:
                tutils.exit("Errors in log file")

    # Print netplugin errors if any and exit
    def errExit(self, str):
        # print erros from netplugin log file
        for node in self.nodes:
            node.checkForNetpluginErrors()

        # exit the script
        tutils.exit(str)
