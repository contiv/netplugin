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

def gen_cluster_store_args():
    if os.environ["CONTIV_CLUSTER_STORE_DRIVER"] == "etcd":
        store_args = " --etcd-endpoints %s " % os.environ["CONTIV_CLUSTER_STORE_URL"]
    else:
        store_args = " --consul-endpoints %s " % os.environ["CONTIV_CLUSTER_STORE_URL"]
    return store_args

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

    # Create v2plugin on vagrant node
    def createV2Plugin(self, args=""):
        ssh_object = self.sshConnect(self.username, self.password)
        command = "docker plugin create " + os.environ.get("CONTIV_V2PLUGIN_NAME", "contiv/v2plugin:0.0") + " /opt/gopath/src/github.com/contiv/netplugin/install/v2plugin/ " + args + ">> /tmp/netplugin.log 2>&1"
        self.runCmd(command)

    # Enable v2plugin on vagrant node
    def enableV2Plugin(self, role="master", args=""):
        ssh_object = self.sshConnect(self.username, self.password)
        fwd_mode = os.environ.get("CONTIV_V2PLUGIN_FWDMODE", "bridge")
        command = "docker plugin set " + os.environ.get("CONTIV_V2PLUGIN_NAME", "contiv/v2plugin:0.0") + " ctrl_ip="+ self.addr + " control_url=" + self.addr + ":9999 vxlan_port=8472 iflist=eth2,eth3 plugin_name=" + os.environ.get("CONTIV_V2PLUGIN_NAME","contiv/v2plugin:0.0") + " fwd_mode=" + fwd_mode + " " + args + " >> /tmp/netplugin.log 2>&1"
        self.runCmd(command)
        command = "docker plugin enable " + os.environ.get("CONTIV_V2PLUGIN_NAME", "contiv/v2plugin:0.0") +  args + " >> /tmp/netplugin.log 2>&1"
        self.npThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        # npThread.setDaemon(True)
        self.npThread.start()

    # Start netplugin process on vagrant node
    def startNetplugin(self, args=""):
        ssh_object = self.sshConnect(self.username, self.password)
        # NOTE: this testing only used in mesos-docker
        mode_args = " --fwdmode bridge --netmode vlan --mode docker "
        command = "sudo " + self.binpath + "/netplugin --vlan-if eth2 --vlan-if eth3 " + mode_args + gen_cluster_store_args() + args + "> /tmp/netplugin.log 2>&1"
        self.npThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        # npThread.setDaemon(True)
        self.npThread.start()

    # Start netmaster process
    def startNetmaster(self):
        ssh_object = self.sshConnect(self.username, self.password)
        # NOTE: this testing only used in mesos-docker
        mode_args = " --fwdmode bridge --netmode vlan --mode docker "
        listenUrlArg = ""
        listenPort = os.environ.get("CONTIV_NETMASTER_LISTEN_PORT","")
        if listenPort:
            listenUrlArg = " --listen-url " + os.environ.get("CONTIV_NETMASTER_LISTEN_IP","") + ":" + listenPort
        ctrlUrlArg = ""
        ctrlPort = os.environ.get("CONTIV_NETMASTER_CONTROL_PORT","")
        if ctrlPort:
            ctrlUrlArg = " --control-url " + os.environ.get("CONTIV_NETMASTER_CONTROL_IP","") + ":" + ctrlPort
        command = "GOPATH=/opt/gopath " + self.binpath + "/netmaster " + listenUrlArg + ctrlUrlArg + mode_args + gen_cluster_store_args() + " > /tmp/netmaster.log 2>&1"
        self.nmThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        # npThread.setDaemon(True)
        self.nmThread.start()

    # Execute command in a thread
    def runCmdThread(self, command):
        ssh_object = self.sshConnect(self.username, self.password)
        self.swThread = threading.Thread(target=ssh_exec_thread, args=(ssh_object, command))
        self.swThread.start()

    # Stop v2plugin by force rm
    def stopV2Plugin(self, args=""):
        command = "docker plugin disable " + os.environ.get("CONTIV_V2PLUGIN_NAME", "contiv/v2plugin:0.0") + "> /tmp/netplugin.log 2>&1"
        command = "docker plugin rm -f " + os.environ.get("CONTIV_V2PLUGIN_NAME", "contiv/v2plugin:0.0") + "> /tmp/netplugin.log 2>&1"
        self.runCmd(command)

    # Stop netplugin by killing it
    def stopNetplugin(self):
        self.runCmd("sudo pkill netplugin")

    # Stop netmaster by killing it
    def stopNetmaster(self):
        self.runCmd("sudo pkill netmaster")

    def cleanupDockerNetwork(self):
        # cleanup docker network
        out, err, exitCode = self.runCmd("docker network ls | grep -w 'netplugin\|contiv' | awk '{print $2}'")
        for net in out:
            self.runCmd("docker network rm " + net)
            time.sleep(1)

    # Remove all containers on this node
    def cleanupContainers(self):
        self.runCmd("docker ps -a | grep -v 'swarm\|CONTAINER ID' | awk '{print $1}' | xargs -r docker rm -fv ")
        self.runCmd("docker service rm `docker service ls -q`")

    # Cleanup all state created by netplugin
    def cleanupSlave(self):
        self.runCmd("docker ps -a | grep alpine | awk '{print $1}' | xargs -r docker rm -fv ")
        self.runCmd("sudo ovs-vsctl list-br | grep contiv | xargs -I % sudo ovs-vsctl del-br % >/dev/null 2>&1")
        self.runCmd("/sbin/ifconfig | grep -e vport | awk '{print $1}' | xargs -r -n1 -I{} sudo ip link delete {} type veth")
        self.runCmd("sudo rm -f /var/run/docker/plugins/netplugin.sock")
        self.runCmd("sudo rm -f /tmp/net*")

    # Cleanup all state created by netmaster
    def cleanupMaster(self):
        self.runCmd("etcdctl ls /contiv > /dev/null 2>&1 && etcdctl rm --recursive /contiv")
        self.runCmd("etcdctl ls /contiv.io > /dev/null 2>&1 && etcdctl rm --recursive /contiv.io")
        self.runCmd("etcdctl ls /docker > /dev/null 2>&1 && etcdctl rm --recursive /docker")
        self.runCmd("curl -X DELETE localhost:8500/v1/kv/contiv.io?recurse=true")
        self.runCmd("curl -X DELETE localhost:8500/v1/kv/docker?recurse=true")

    # Run container on a node
    def runContainer(self, imgName="ubuntu:14.04", cmdName="sh", networkName=None, serviceName=None, cntName=""):
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

    def checkForNetpluginErrors(self):
        out, err, exitCode = self.runCmd('grep "error\|fatal" /tmp/net*')
        if out != [] or err != []:
            print "\n\n\n\n\n\n"
            tutils.log("Error:\n" + ''.join(out) + ''.join(err))
            tutils.log("Errors seen in log files on: " + self.hostname)
            return False

        return True
