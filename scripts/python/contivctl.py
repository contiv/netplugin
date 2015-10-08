#!/usr/bin/python

import urllib
import urllib2
import json
import argparse
import os

# HTTP Delete wrapper
def httpDelete(url):
	opener = urllib2.build_opener(urllib2.HTTPHandler)
	request = urllib2.Request(url)
	request.get_method = lambda: 'DELETE'
	try:
		url = opener.open(request)

	except urllib2.HTTPError, err:
		if err.code == 404:
			print "Page not found!"
		elif err.code == 403:
			print "Access denied!"
		else:
			print "HTTP Error response! Error code", err.code

	except urllib2.URLError, err:
		print "URL error:", err.reason

# HTTP POST wrapper
def httpPost(url, data):
	try:
		retData = urllib2.urlopen(url, data)
		return retData.read()
	except urllib2.HTTPError, err:
		if err.code == 404:
			print "Page not found!"
		elif err.code == 403:
			print "Access denied!"
		else:
			print "HTTP Error! Error code", err.code
		return "Error"
	except urllib2.URLError, err:
		print "URL error:", err.reason
		return "Error"

# Wrapper for HTTP get
def httpGet(url):
    try:
        retData = urllib2.urlopen(url)
        return retData.read()

    except urllib2.HTTPError, err:
        if err.code == 404:
            print "Page not found!"
        elif err.code == 403:
            print "Access denied!"
        else:
            print "HTTP Error! Error code", err.code
        os._exit(1)
    except urllib2.URLError, err:
        print "URL error:", err.reason
        os._exit(1)

# Create policy
def createPolicy(args):
	print "Creating policy {0}:{1}".format(args.tenantName, args.policyName)
	postUrl = 'http://netmaster:9999/api/policys/' + args.tenantName + ':' + args.policyName + '/'
	jdata = json.dumps({
	  "tenantName": args.tenantName,
	  "policyName": args.policyName
	 })
	response = httpPost(postUrl, jdata)
	print "Create policy response is: " + response

# Delete policy
def deletePolicy(args):
	print "Deleting policy {0}:{1}".format(args.tenantName, args.policyName)

	# Delete Policy
	deleteUrl = 'http://netmaster:9999/api/policys/' + args.tenantName + ':' + args.policyName + '/'
	httpDelete(deleteUrl)

# List all policies
def listPolicy(args):
	print "Listing all policies for tenant {0}".format(args.tenantName)

	# Get a list of policies
	policyList = json.loads(httpGet('http://netmaster:9999/api/policys/'))

	print "Tenant,		Policy"
	print "-----------------------------------"
	# Print each policy for the tenant
	for policy in policyList:
		if policy['tenantName'] == args.tenantName:
			print "{0}		{1}".format(policy['tenantName'], policy['policyName'])


# Add rule to a policy
def addRule(args):
	print "Adding rule to pilicy rule {0}:{1}".format(args.tenantName, args.policyName)

	#Post the data
	postUrl = 'http://netmaster:9999/api/rules/' + args.tenantName + ':' + args.policyName + ':' + args.ruleId + '/'
	jdata = json.dumps({
	  "tenantName": args.tenantName,
	  "policyName": args.policyName,
	  "ruleId": args.ruleId,
	  "priority": args.priority,
	  "direction": args.direction,
	  "endpointGroup": args.endpointGroup,
	  "network": args.network,
	  "ipAddress": args.ipAddress,
	  "protocol": args.protocol,
	  "port": int(args.port),
	  "action": args.action
	 })
	print "rule create, sending: " + jdata
	response = httpPost(postUrl, jdata)
	print "Rule add response is: " + response

# Delete rule
def deleteRule(args):
	print "Deleting rule {0}:{1}:{2}".format(args.tenantName, args.policyName, args.ruleId)

	# Delete Rule
	deleteUrl = 'http://netmaster:9999/api/rules/' + args.tenantName + ':' + args.policyName + ':' + args.ruleId + '/'
	httpDelete(deleteUrl)

# List all rules
def listRule(args):
	print "Listing all rules for policy {0}:{1}".format(args.tenantName, args.policyName)

	# Get the list of all rules
	ruleList = json.loads(httpGet('http://netmaster:9999/api/rules/'))

	print "Rule, direction, priority, endpointGroup, network, ipAddress, protocol, port, action"
	print "---------------------------------------------------------------------------------------------"

	for rule in ruleList:
		if rule['tenantName'] == args.tenantName and rule['policyName'] == args.policyName:

			# Handle if field not present
			if 'endpointGroup' not in rule:
				rule['endpointGroup'] = "--"
			if 'network' not in rule:
				rule['network'] = "--"
			if 'ipAddress' not in rule:
				rule['ipAddress'] = "--"
			if 'protocol' not in rule:
				rule['protocol'] = "--"
			if 'port' not in rule:
				rule['port'] = "--"

			print "{0}, {1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}".format(rule['ruleId'], rule['direction'], rule['priority'], rule['endpointGroup'], rule['network'], rule['ipAddress'], rule['protocol'], rule['port'], rule['action'])


# Create endpoint group
def createEpg(args):
	print "Creating endpoint group {0}:{1}".format(args.tenantName, args.groupName)

	# Create epg
	postUrl = 'http://netmaster:9999/api/endpointGroups/' + args.tenantName + ':' + args.groupName + '/'
	jdata = json.dumps({
	  "tenantName": args.tenantName,
	  "groupName": args.groupName,
	  "networkName": args.networkName,
	  "policies": args.policies.split(",") if args.policies != "" else [],
	 })
	response = httpPost(postUrl, jdata)
	print "Epg Create response is: " + response

# Delete endpoint group
def deleteEpg(args):
	print "Deleting endpoint group {0}:{1}".format(args.tenantName, args.groupName)

	# Delete EPG
	deleteUrl = 'http://netmaster:9999/api/endpointGroups/' + args.tenantName + ':' + args.groupName + '/'
	httpDelete(deleteUrl)

# List all endpoint groups
def listEpg(args):
	print "Listing all endpoint groups for tenant {0}".format(args.tenantName)

	# Get the list of endpoint groups
	epgList = json.loads(httpGet('http://netmaster:9999/api/endpointGroups/'))

	print "Group		Network		Policies"
	print "---------------------------------------------------"

	# Print epgs
	for epg in epgList:
		if epg['tenantName'] == args.tenantName:

			# Handle empty fields
			network = epg['networkName'] if 'networkName' in epg else "--"
			policies = ",".join(epg['policies']) if 'policies' in epg else "--"

			print "{0}		{1}		{2}".format(epg['groupName'], network, policies)

# Create Network
def createNet(args):
	print "Creating network {0}:{1}".format(args.tenantName, args.networkName)

	# Create network
	postUrl = 'http://netmaster:9999/api/networks/' + args.tenantName + ':' + args.networkName + '/'
	jdata = json.dumps({
	  "tenantName": args.tenantName,
	  "networkName": args.networkName,
	  "isPublic": True if args.public == True else False,
	  "isPrivate": False if args.public == True else True,
	  "encap": args.encap,
	  "subnet": args.subnet,
	  "defaultGw": args.defaultGw,
	 })
	response = httpPost(postUrl, jdata)
	print "Network Create response is: " + response

# Delete Network
def deleteNet(args):
	print "Deleting network {0}:{1}".format(args.tenantName, args.networkName)

	# Delete network
	deleteUrl = 'http://netmaster:9999/api/networks/' + args.tenantName + ':' + args.networkName + '/'
	httpDelete(deleteUrl)

# List all Networks
def listNet(args):
	print "Listing all networks for tenant {0}".format(args.tenantName)

	# Get the list of Networks
	netList = json.loads(httpGet('http://netmaster:9999/api/networks/'))

	print "Network		Public	Encap	Subnet			Gateway"

	# Print the networks
	for net in netList:
		if net['tenantName'] == args.tenantName:
			isPublic = "No"
			if 'isPublic' in net and net['isPublic'] == True:
				isPublic = "Yes"

			print "{0}		{1}	{2}	{3}		{4}".format(net['networkName'], isPublic, net['encap'], net['subnet'], net['defaultGw'])


# Add policy subparser
def addPolicyParser(sub):
	policyParser = sub.add_parser("policy", help="Policy operations")
	policySubparser = policyParser.add_subparsers()

	# Add policy add/delete commands
	plCreateParser = policySubparser.add_parser("create", help="Create policy")
	plDeleteParser = policySubparser.add_parser("delete", help="Delete policy")
	plListParser = policySubparser.add_parser("list", help="List all policies")

	# Policy name
	plCreateParser.add_argument("policyName", help="Policy name")
	plDeleteParser.add_argument("policyName", help="Policy name")

	# tenant name
	plCreateParser.add_argument("-tenantName", default="default")
	plDeleteParser.add_argument("-tenantName", default="default")
	plListParser.add_argument("-tenantName", default="default")

	# Handler functions
	plCreateParser.set_defaults(func=createPolicy)
	plDeleteParser.set_defaults(func=deletePolicy)
	plListParser.set_defaults(func=listPolicy)


# Add Rule parser
def addRuleParser(sub):
	ruleParser = sub.add_parser("rule", help="Rule add/delete")
	ruleSubparser = ruleParser.add_subparsers()

	# Add rule add/delete commands
	ruleAddParser = ruleSubparser.add_parser("add", help="Add rule to a policy")
	ruleDeleteParser = ruleSubparser.add_parser("delete", help="Delete rule from a policy")
	ruleListParser = ruleSubparser.add_parser("list", help="List all rules for a policy")

	# Policy name
	ruleAddParser.add_argument("policyName", help="Policy name")
	ruleDeleteParser.add_argument("policyName", help="Policy name")
	ruleListParser.add_argument("policyName", help="Policy name")

	# Rule Id
	ruleAddParser.add_argument("ruleId", help="Rule identifier")
	ruleDeleteParser.add_argument("ruleId", help="Rule identifier")

	# Tenant name
	ruleAddParser.add_argument("-tenantName", default="default")
	ruleDeleteParser.add_argument("-tenantName", default="default")
	ruleListParser.add_argument("-tenantName", default="default")

	# Rule Parameters
	ruleAddParser.add_argument("-direction", default="in", choices=["in", "out", "both"])
	ruleAddParser.add_argument("-priority", type=int, default=1, help="priority [1..100]")
	ruleAddParser.add_argument("-endpointGroup", help="Name of endpoint group")
	ruleAddParser.add_argument("-network", help="Name of network")
	ruleAddParser.add_argument("-ipAddress", help="IP address/mask")
	ruleAddParser.add_argument("-protocol", default="", choices=["tcp", "udp", "icmp", "igmp"], help="IP protocol")
	ruleAddParser.add_argument("-port", default="0", help="tcp/udp port number")
	ruleAddParser.add_argument("-action", default="accept", choices=["accept", "deny"], help="Accept or deny")

	# Handler functions
	ruleAddParser.set_defaults(func=addRule)
	ruleDeleteParser.set_defaults(func=deleteRule)
	ruleListParser.set_defaults(func=listRule)

# Add EPG parser
def addEpgParser(sub):
	epgParser = sub.add_parser("group", help="Endpoint group operations")
	epgSubparser = epgParser.add_subparsers()

	# Add EPG add/delete commands
	epgCreateParser = epgSubparser.add_parser("create", help="Create endpoint group")
	epgDeleteParser = epgSubparser.add_parser("delete", help="Delete endpoint group")
	epgListParser = epgSubparser.add_parser("list", help="List all endpoint groups")

	# Group name
	epgCreateParser.add_argument("groupName", help="Endpoint group name")
	epgDeleteParser.add_argument("groupName", help="Endpoint group name")

	# Tenant name
	epgCreateParser.add_argument("-tenantName", default="default")
	epgDeleteParser.add_argument("-tenantName", default="default")
	epgListParser.add_argument("-tenantName", default="default")

	# Epg params
	epgCreateParser.add_argument("-networkName", help="Network name")
	epgCreateParser.add_argument("-policies", default="", help="List of policies")

	# Handler functions
	epgCreateParser.set_defaults(func=createEpg)
	epgDeleteParser.set_defaults(func=deleteEpg)
	epgListParser.set_defaults(func=listEpg)

# Add network parser
def addNetworkParser(sub):
	netParser = sub.add_parser("network", help="Network operations")
	netSubparser = netParser.add_subparsers()

	#Add network add/delete commands
	netCreateParser = netSubparser.add_parser("create", help="Create Network")
	netDeleteParser = netSubparser.add_parser("delete", help="Delete Network")
	netListParser = netSubparser.add_parser("list", help="List all Networks")

	# Network name
	netCreateParser.add_argument("networkName", help="Network name")
	netDeleteParser.add_argument("networkName", help="Network name")

	# Tenant name
	netCreateParser.add_argument("-tenantName", default="default")
	netDeleteParser.add_argument("-tenantName", default="default")
	netListParser.add_argument("-tenantName", default="default")

	# Network params
	netCreateParser.add_argument("-public", default="no", choices=["yes", "no"], help="Is this a public network")
	netCreateParser.add_argument("-encap", default="vxlan", choices=["vlan", "vxlan"], help="Packet tag")
	netCreateParser.add_argument("-subnet", required=True, help="Subnet addr/mask")
	netCreateParser.add_argument("-defaultGw", required=True, help="default GW")

	# Handler functions
	netCreateParser.set_defaults(func=createNet)
	netDeleteParser.set_defaults(func=deleteNet)
	netListParser.set_defaults(func=listNet)

# Create the parser and sub parser
parser = argparse.ArgumentParser()
parser.add_argument('--version', action='version', version='1.0.0')
subparsers = parser.add_subparsers()

# Add subparser for each object
addPolicyParser(subparsers)
addRuleParser(subparsers)
addEpgParser(subparsers)
addNetworkParser(subparsers)

# Run the parser
args = parser.parse_args()
args.func(args)  # call the default function
