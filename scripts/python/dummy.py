#!/usr/bin/python

import utils
import time
import sys

# Test routines to test the python code
node = utils.vagrantNode("192.168.2.10")
cnt1 = node.runContainer()
ipAddr = cnt1.getIpAddr()
print "IP address: " + ipAddr
cnt2 = node.runContainer()
cnt2.checkPing(ipAddr)
cnt2.checkPing("192.168.2.20")

# cnt1.remove()
