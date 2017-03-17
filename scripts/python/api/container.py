import tutils
import time

# This class represents a docker Container
class Container:
    def __init__(self, node, cid, cntName=""):
        self.node = node
        self.cid = cid
        self.cntName = cntName

        # Read interface ip
        out, err, exitCode = self.execCmd('ip addr show dev eth0 | grep "inet "')
        if err != [] or exitCode != 0:
            print "Error during docker exec"
            print err
            tutils.exit("Failed to get IP address")

        self.eth0Addr = out[0].strip().split(' ')[1].split('/')[0]

    # Return container identifier
    def myId(self):
        if self.cntName != "":
            return self.node.hostname + "(" + self.node.addr + ")/" + self.cntName
        else:
            return self.node.hostname + "(" + self.node.addr + ")/" + self.cid

    def errorExit(self, str, out, err):
        print str + " " + self.myId()
        print "Output: " + ''.join(out)
        print "Error: " + ''.join(err)
        tutils.exit(str)

    # Execute a command inside a container
    def execCmd(self, cmd):
        out, err, exitCode = self.node.runCmd("docker exec " + self.cid + " " + cmd)
        # Retry failures once to workaround docker issue #15713
        if out == [] and err == [] and exitCode == 255:
            time.sleep(1)
            return self.node.runCmd("docker exec " + self.cid + " " + cmd)
        return out, err, exitCode

    # Execute a command inside a container in background
    def execBgndCmd(self, cmd):
        out, err, exitCode = self.node.runCmd("docker exec -d " + self.cid + " " + cmd)
        # Retry failures once to workaround docker issue #15713
        if out == [] and err == [] and exitCode == 255:
            time.sleep(1)
            return self.node.runCmd("docker exec -d " + self.cid + " " + cmd)
        return out, err, exitCode

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

    # Get IP address of the container
    def getIpAddr(self, intfName="eth0"):
        if intfName == "eth0":
            return self.eth0Addr

        # Read interface ip
        out, err, exitCode = self.execCmd("ifconfig " + intfName)
        if err != [] or exitCode != 0:
            print "Error during docker exec"
            print err
            tutils.exit("Failed to get IP address")

        return out[1].split('addr')[1].split(' ')[0].split(':')[1]

    def checkPing(self, ipAddr):
        tutils.log("Checking ping from " + self.myId() + " to " + ipAddr)
        out, err, exitCode = self.execCmd("ping -c 5 -i 0.2 " + ipAddr)
        if err != [] or exitCode != 0:
            print "ping exit(" + str(exitCode) + ") Output: " + ''.join(out)
            print "Error during ping"
            print err
            tutils.exit("Ping failed")

        # Check if ping succeeded
        pingOutput = ''.join(out)
        if "0 received, 100% packet loss" in pingOutput:
            print "Ping failed. Output: " + pingOutput
            tutils.exit("Ping failed")

        tutils.log("ping from " + self.myId() + " to " + ipAddr + " Successful!")
        return True

    def checkPingFailure(self, ipAddr):
        tutils.log("Checking ping failure from " + self.myId() + " to " + ipAddr)
        out, err, exitCode = self.execCmd("ping -c 5 -i 0.5 -W 1 " + ipAddr)
        pingOutput = ''.join(out)
        if err != [] or exitCode != 0:
            print "ping exit(" + str(exitCode) + ") Output: " + pingOutput
            tutils.log("Ping failed as expected.")
            return True

        # Check if ping succeeded
        if "0 received, 100% packet loss" in pingOutput:
            tutils.log("Ping failed as expected. Output: " + pingOutput)
            return True

        tutils.log("ping from " + self.myId() + " to " + ipAddr + " succeeded while expecting failure")
        tutils.exit("Ping succeeded while expecting failure")

    # Start netcast listener on container
    def startListener(self, port, protocol="tcp"):
        protoStr = "-u " if protocol == "udp" else " "
        out, err, exitCode = self.execBgndCmd("netcat -k -l " + protoStr + "-p " + str(port))
        if exitCode != 0:
            self.errorExit("Error starting netcat", out, err)

    def stopListener(self):
        out, err, exitCode = self.execCmd("pkill netcat")
        if exitCode != 0:
            tutils.info("Error stopping netcat".join(out).join(err))

    # Check if this container can connect to destination port
    def checkConnection(self, ipAddr, port, protocol="tcp"):
        tutils.log("Checking connection from " + self.myId() + " to " + ipAddr + " port " + str(port))
        protoStr = "-u " if protocol == "udp" else " "
        out, err, exitCode = self.execCmd("netcat -z -n -v -w 1 " + protoStr + ipAddr + " " + str(port))

        print "checkConnection Output(" + str(exitCode) + "): " + ''.join(out)
        print "checkConnection Err: " + ''.join(err)

        ncOut = ''.join(out) + ''.join(err)
        if "succeeded" in ncOut and exitCode == 0:
            return True
        else:
            return False
