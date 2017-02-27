#!/usr/bin/env python

# Start netplugin and netmaster
import api.tbed
import time
import sys
import os
import argparse

# Parse command line args
# Create the parser and sub parser
parser = argparse.ArgumentParser()
parser.add_argument('--version', action='version', version='1.0.0')
parser.add_argument("-nodes", required=True, help="list of nodes(comma separated)")
parser.add_argument("-user", default='vagrant', help="User id for ssh")
parser.add_argument("-password", default='vagrant', help="password for ssh")
parser.add_argument("-binpath", default='/opt/gopath/bin', help="netplugin/netmaster binary path")
parser.add_argument("-plugintype", default='binary', help="Docker v2-plugin name")

# Parse the args
args = parser.parse_args()
addrList = args.nodes.split(",")

# Cleanup all state and start netplugin/netmaster
testbed = api.tbed.Testbed(addrList, args.user, args.password, args.binpath, args.plugintype)

print "Waiting for netmaster to come up"
time.sleep(15)

print "################### Started Netplugin #####################"
os._exit(0)
