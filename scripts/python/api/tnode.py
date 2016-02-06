import paramiko
import threading
import sys
import os
import time
import container
import tutils
import exceptions

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

# This class represents a vagrant node
class Node:
    def __init__(self, addr, username='vagrant', password='vagrant', binpath='/opt/gopath/bin'):
        self.addr = addr
        self.username = username
        self.password = password
        self.binpath = binpath
        self.ssh = self.sshConnect(username, password)
        out, err, ec = self.runCmd("hostname")
        self.hostname = out[0].split('\n')[0]
        print "Connected to " + self.hostname

    # Connect to vagrant node
    def sshConnect(self, username, password):
        ssh_object = paramiko.SSHClient()
        ssh_object.set_missing_host_key_policy( paramiko.AutoAddPolicy() )
        print "Connecting to " + self.addr + " with userid: " + username + " password: " + password
        try:
            ssh_object.connect(self.addr, username=username, password=password)
            return ssh_object
        except paramiko.ssh_exception.AuthenticationException:
            tutils.exit("Authentication failed")

    def isConnected(self):
        transport = self.ssh.get_transport() if self.ssh else None
        return transport and transport.is_active()

    # Run a command on vagrant node
    def runCmd(self, cmd, timeout=None):
        try:
            print "run: " + cmd
            # We we disconnected for any reason, reconnect
            if not self.isConnected():
                self.ssh = self.sshConnect(self.username, self.password)

            # Execute the command
            stdin, stdout, stderr = self.ssh.exec_command(cmd, timeout=timeout)
            out = stdout.readlines()
            err = stderr.readlines()
            exitCode = stdout.channel.recv_exit_status()
            if out != [] or exitCode != 0:
                print "stdout(" + str(exitCode) + "):" + ''.join(out)
            if err != []:
                print "stderr: " + ''.join(err)
            return out, err, exitCode
        except exceptions.EOFError:
            print "Ignoring EOF errors executing command"
            return [], [], 0

    # Start netplugin process on vagrant node
    def startNetplugin(self, args=""):
        ssh_object = self.sshConnect(self.username, self.password)
        command = "sudo " + self.binpath + "/netplugin -plugin-mode docker -vlan-if eth2 " + args + "> /tmp/netplugin.log 2>&1"
        self.npThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        # npThread.setDaemon(True)
        self.npThread.start()

    # Start netmaster process
    def startNetmaster(self):
        ssh_object = self.sshConnect(self.username, self.password)
        command = "GOPATH=/opt/gopath " + self.binpath + "/netmaster > /tmp/netmaster.log 2>&1"
        self.nmThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        # npThread.setDaemon(True)
        self.nmThread.start()

    # Stop netplugin by killing it
    def stopNetplugin(self):
        self.runCmd("sudo pkill netplugin")

    # Stop netmaster by killing it
    def stopNetmaster(self):
        self.runCmd("sudo pkill netmaster")

    def cleanupDockerNetwork(self):
        # cleanup docker network
        out, err, exitCode = self.runCmd("docker network ls | grep netplugin | awk '{print $2}'")
        for net in out:
            self.runCmd("docker network rm " + net)

    # Remove all containers on this node
    def cleanupContainers(self):
        self.runCmd("docker rm -f `docker ps -aq`")

    # Cleanup all state created by netplugin
    def cleanupSlave(self):
        self.runCmd("docker rm -f `docker ps -aq`")
        self.runCmd("sudo ovs-vsctl del-br contivVxlanBridge")
        self.runCmd("sudo ovs-vsctl del-br contivVlanBridge")
        self.runCmd("for p in `ifconfig  | grep vport | awk '{print $1}'`; do sudo ip link delete $p type veth; done")
        self.runCmd("sudo rm /var/run/docker/plugins/netplugin.sock")
        self.runCmd("sudo rm /tmp/net*")
        self.runCmd("sudo service docker restart")

    # Cleanup all state created by netmaster
    def cleanupMaster(self):
        self.runCmd("etcdctl rm --recursive /contiv")
        self.runCmd("etcdctl rm --recursive /contiv.io")
        self.runCmd("etcdctl rm --recursive /docker")
        self.runCmd("etcdctl rm --recursive /skydns")

    # Run container on a node
    def runContainer(self, imgName="ubuntu", cmdName="sh", networkName=None, serviceName=None, cntName=""):
        netSrt = ""
        if networkName != None:
            netSrt = "--net=" + networkName
            if serviceName != None:
                netSrt = "--net=" + serviceName + "." + networkName
        cntStr = ""
        if cntName != "":
            cntStr = "--name=" + cntName

        # docker command
        dkrCmd = "docker run -itd " + netSrt + " " + cntStr + " " + imgName + " " + cmdName
        out, err, exitCode = self.runCmd(dkrCmd)
        if exitCode != 0:
            print "Error running container: " + dkrCmd + " on " + self.addr
            print "Exit status: " + str(exitCode) + "\nError:"
            print err
            exit("docker run failed")

        # Container id is in the first line
        cid = out[0].split('\n')[0]

        # Return a container object
        return container.Container(self, cid, cntName="")

    def chekForNetpluginErrors(self):
        out, err, exitCode = self.runCmd('grep "error\|fatal" /tmp/net*')
        if out != [] or err != []:
            print "\n\n\n\n\n\n"
            tutils.log("Error:\n" + ''.join(out) + ''.join(err))
            tutils.log("Errors seen in log files on: " + self.hostname)
            return False

        return True
