# Python testing utility classes and functions
import paramiko
import threading
import sys
import os

# Utility function to run ssh
def ssh_exec_thread(ssh_object, command):
    print "run: " + command
    stdin, stdout, stderr = ssh_object.exec_command(command)
    out = stdout.readlines()
    print out
    print "Program exited: " + command
    exitCode = stdout.channel.recv_exit_status()
    if exitCode != 0:
        print "Exit code: " + str(exitCode)

def info(str):
    print "###################### " + str + " ######################"

def log(str):
    print "######## " + str

def exit(str):
    info("Test failed: " + str)
    os._exit(1)

# This class represents a vagrant node
class vagrantNode:
    def __init__(self, addr):
        self.addr = addr
        self.ssh = self.sshConnect()
        out, err, ec = self.runCmd("hostname")
        self.hostname = out[0].split('\n')[0]
        print "Connected to " + self.hostname

    # Connect to vagrant node
    def sshConnect(self):
        ssh_object = paramiko.SSHClient()
        ssh_object.set_missing_host_key_policy( paramiko.AutoAddPolicy() )
        ssh_object.connect(self.addr, username='vagrant', password='vagrant')
        return ssh_object

    # Run a command on vagrant node
    def runCmd(self, cmd, timeout=None):
        print "run: " + cmd
        stdin, stdout, stderr = self.ssh.exec_command(cmd, timeout=timeout)
        out = stdout.readlines()
        err = stderr.readlines()
        exitCode = stdout.channel.recv_exit_status()
        if out != [] or exitCode != 0:
            print "stdout(" + str(exitCode) + "):" + ''.join(out)
        if err != []:
            print "stderr: " + ''.join(err)
        return out, err, exitCode

    # Start netplugin process on vagrant node
    def startNetplugin(self):
        ssh_object = self.sshConnect()
        command = "sudo /opt/gopath/bin/netplugin -native-integration=true > /tmp/netplugin.log 2>&1"
        self.npThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        # npThread.setDaemon(True)
        self.npThread.start()

    # Start netmaster process
    def startNetmaster(self):
        ssh_object = self.sshConnect()
        command = "GOPATH=/opt/gopath /opt/gopath/bin/netmaster > /tmp/netmaster.log 2>&1"
        self.nmThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        # npThread.setDaemon(True)
        self.nmThread.start()

    # Stop netplugin by killing it
    def stopNetplugin(self):
        self.runCmd("sudo killall netplugin")

    # Stop netmaster by killing it
    def stopNetmaster(self):
        self.runCmd("sudo killall netmaster")

    # Cleanup all state created by netplugin
    def cleanupSlave(self):
        self.runCmd("docker rm -f `docker ps -aq`")
        self.runCmd("sudo ovs-vsctl del-br contivVxlanBridge")
        self.runCmd("sudo ovs-vsctl del-br contivVlanBridge")
        self.runCmd("for p in `ifconfig  | grep vport | awk '{print $1}'`; do sudo ip link delete $p type veth; done")
        self.runCmd("sudo rm /var/run/docker/plugins/netplugin.sock")
        self.runCmd("sudo service docker restart")

    # Cleanup all state created by netmaster
    def cleanupMaster(self):
        self.runCmd("etcdctl rm --recursive /contiv")
        self.runCmd("etcdctl rm --recursive /contiv.io")

    def runContainer(self, imgName="ubuntu", cmdName="sh", serviceName=None):
        srvStr = ""
        if serviceName != None:
            srvStr = "--publish-service " + serviceName
        # docker command
        dkrCmd = "docker run -itd " + srvStr + " " + imgName + " " + cmdName
        out, err, exitCode = self.runCmd(dkrCmd)
        if exitCode != 0:
            print "Error running container: " + dkrCmd + " on " + self.addr
            print "Exit status: " + str(exitCode) + "\nError:"
            print err
            exit("docker run failed")

        # Container id is in the first line
        cid = out[0].split('\n')[0]

        # Return a container object
        return container(self, cid, serviceName)

# This class represents a testbed i.e, collection of vagrant nodes
class testbed:
    # Initialize a testbed
    def __init__(self, addrList):
        self.nodes = []
        # Basic error checking
        if len(addrList) < 1:
            print "Empty address list"
            sys.exit(1)

        # Create nodes
        for addr in addrList:
            node = vagrantNode(addr)
            self.nodes.append(node)

            # Cleanup all state before we can start
            node.stopNetmaster()
            node.stopNetplugin()

        # cleanup master and slave state
        for node in self.nodes:
            node.cleanupMaster()
            node.cleanupSlave()

        # Start netmaster
        print "Starting netmaster"
        self.nodes[0].startNetmaster()

        # Start netplugin on all nodes
        for node in self.nodes:
            print "Starting netplugin on " + node.addr
            node.startNetplugin()

    # Cleanup a testbed once test is done
    def cleanup(self):
        # Cleanup each node
        for node in self.nodes:
            print "Cleaning up node " + node.addr
            node.stopNetplugin()
            node.cleanupSlave()

        # cleanup master
        print "Cleaning up master"
        self.nodes[0].stopNetmaster()
        self.nodes[0].cleanupMaster()

    # Number of nodes in the testbed
    def numNodes(self):
        return len(self.nodes)

    # Start containers on the testbed
    def runContainers(self, numContainer):
        containers = []
        # Start the containers
        for cntIdx in range(numContainer):
            nodeIdx = cntIdx % self.numNodes()
            srvName = "srv" + str(cntIdx) + ".private"
            cnt = self.nodes[nodeIdx].runContainer("ubuntu", serviceName=srvName)
            containers.append(cnt)

        return containers

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

        # Test ping to all other containers
        for cnt in containers:
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

        # Check connection to all containers
        for cidx, cnt in enumerate(containers):
            for aidx, ipAddr in enumerate(cntrIpList):
                if cidx != aidx:
                    ret = cnt.checkConnection(ipAddr, port)
                    # If connection status is not what we were expecting, we are done.
                    if ret != success:
                        return ret

        # Return
        return success


# This class represents a docker Container
class container:
    def __init__(self, node, cid, serviceName=None):
        self.node = node
        self.cid = cid
        self.serviceName = serviceName

    # Return container identifier
    def myId(self):
        if self.serviceName != None:
            return self.node.hostname + "(" + self.node.addr + ")/" + self.serviceName
        else:
            return self.node.hostname + "(" + self.node.addr + ")/" + self.cid

    def errorExit(self, str, out, err):
        print str + " " + self.myId()
        print "Output: " + ''.join(out)
        print "Error: " + ''.join(err)
        exit(str)

    # Execute a command inside a container
    def execCmd(self, cmd):
        return self.node.runCmd("docker exec " + self.cid + " " + cmd)

    # Execute a command inside a container in backgroud
    def execBgndCmd(self, cmd):
        return self.node.runCmd("docker exec -d " + self.cid + " " + cmd)

    # start the container
    def start(self):
        out, err, exitCode = self.node.runCmd("docker start " + self.cid)
        if exitCode != 0:
            self.errorExit("Error starting container", out, err)

    # start the container
    def stop(self):
        out, err, exitCode = self.node.runCmd("docker stop " + self.cid)
        if exitCode != 0:
            self.errorExit("Error stopping container", out, err)

    # remove the container
    def remove(self):
        # force remove the container
        out, err, exitCode = self.node.runCmd("docker rm -f " + self.cid)
        if exitCode != 0:
            self.errorExit("Error removing container", out, err)

        # Unpublish the service
        if self.serviceName != None:
            self.node.runCmd("docker service unpublish " + self.serviceName)

    # Get IP address of the container
    def getIpAddr(self, intfName="eth0"):
        # Read interface ip
        out, err, exitCode = self.execCmd("ifconfig " + intfName)
        if err != [] or exitCode != 0:
            print "Error during docker exec"
            print err
            exit("Failed to get IP address")

        return out[1].split('addr')[1].split(' ')[0].split(':')[1]

    def checkPing(self, ipAddr):
        log("Checking ping from " + self.myId() + " to " + ipAddr)
        out, err, exitCode = self.execCmd("ping -c 5 -i 0.2 " + ipAddr)
        if err != [] or exitCode != 0:
            print "Error during ping"
            print err
            exit("Ping failed")

        # Check if ping succeded
        pingOutput = ''.join(out)
        if "0 received, 100% packet loss" in pingOutput:
            print "Ping failed. Output: " + pingOutput
            exit("Ping failed")

        log("ping from " + self.myId() + " to " + ipAddr + " Successful!")
        return True

    # Start netcast listener on container
    def startListener(self, port, protocol="tcp"):
        protoStr = "-u " if protocol == "udp" else " "
        out, err, exitCode = self.execBgndCmd("netcat -k -l -p " + protoStr + str(port))
        if exitCode != 0:
            self.errorExit("Error starting netcat", out, err)

    def stopListener(self):
        out, err, exitCode = self.execCmd("pkill netcat")
        if exitCode != 0:
            info("Error stopping netcat".join(out).join(err))

    # Check if this container can connect to destination port
    def checkConnection(self, ipAddr, port, protocol="tcp"):
        protoStr = "-u " if protocol == "udp" else " "
        out, err, exitCode = self.execCmd("netcat -z -n -v -w 5 " + protoStr + ipAddr + " " + str(port))

        print "checkConnection Output(" + str(exitCode) + "): " + ''.join(out)
        print "checkConnection Err: " + ''.join(err)

        ncOut = ''.join(out) + ''.join(err)
        if "succeeded" in ncOut and exitCode == 0:
            return True
        else:
            return False
