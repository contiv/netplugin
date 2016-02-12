import urllib
import urllib2
import json
import argparse
import os
import http

# contivModel REST client



# Exit on error
def errorExit(str):
    print "############### Test failed: " + str + " ###############"
    os._exit(1)

class tenant:
    def __init__(self, tenantName):
        tenantList = listTenant()
        found = False
        for tnt in tenantList:
            if tnt['tenantName'] == tenantName:
                found = True

        #Create the tenant if not found
        if found == False:
            createTenant(tenantName)

        # Save Parameters
        self.tenantName = tenantName
        self.policies = {}
        self.networks = {}

    # get a network
    def network(self, networkName):
        net = network(self.tenantName, networkName)

        # Store the network
        self.networks[networkName] = net

        return net

    # Create a network
    def newNetwork(self, networkName, pktTag, subnet, gateway, encap="vxlan"):
        net = network(self.tenantName, networkName, pktTag, subnet, gateway, encap)

        # Store the network
        self.networks[networkName] = net

        return net

    # Delete network
    def deleteNetwork(self, networkName):
        if networkName not in self.networks:
            return

        # delete the network
        self.networks[networkName].delete()
        del self.networks[networkName]

    # Create a policy
    def newPolicy(self, policyName):
        pl = policy(self.tenantName, policyName)

        # store the policy
        self.policies[policyName] = pl

        return pl

    # Delete policy
    def deletePolicy(self, policyName):
        if policyName not in self.policies:
            return

        # delete the policy
        self.policies[policyName].delete()
        del self.policies[policyName]

    def delete(self):
        deleteTenant(self.tenantName)


class policy:
    def __init__(self, tenantName, policyName):
        # create policy
        createPolicy(tenantName, policyName)

        # save Parameters
        self.tenantName = tenantName
        self.policyName = policyName
        self.rules = {}

    def delete(self):
        deletePolicy(self.tenantName, self.policyName)

    # create new rule
    def addRule(self, ruleId, priority=1, direction='in',
        endpointGroup="", network="", ipAddress="", protocol="", port=0, action="allow"):
        rl = rule(self.tenantName, self.policyName, ruleId, priority, direction,
            endpointGroup, network, ipAddress, protocol, port, action)

        # store it
        self.rules[ruleId] = rl

        return rl
    def deleteRule(self, ruleId):
        if ruleId not in self.rules:
            return

        # Delete the rule
        self.rules[ruleId].delete()
        del self.rules[ruleId]

class rule:
    def __init__(self, tenantName, policyName, ruleId, priority, direction,
        endpointGroup, network, ipAddress, protocol, port, action):
        # add the rule
        addRule(tenantName, policyName, ruleId, priority, direction,
            endpointGroup, network, ipAddress, protocol, port, action)

        # save Parameters
        self.tenantName = tenantName
        self.policyName = policyName
        self.ruleId = ruleId
        self.priority = priority
        self.direction = direction
        self.endpointGroup = endpointGroup
        self.network = network
        self.ipAddress = ipAddress
        self.protocol = protocol
        self.port = port
        self.action = action

    def delete(self):
        # delete the Rule
        deleteRule(self.tenantName, self.policyName, self.ruleId)

class group:
    def __init__(self, tenantName, networkName, groupName, policies):
        # create the Epg
        createEpg(tenantName, networkName, groupName, policies)

        # save parameters
        self.tenantName = tenantName
        self.groupName = groupName
        self.networkName = networkName
        self.policies = policies

    #add policy to the Group
    def addPolicy(self, policyName):
        # add the policy to the list
        self.policies.append(policyName)

        # update the model
        createEpg(self.tenantName, self.networkName, self.groupName, self.policies)

    def removePolicy(self, policyName):
        # check if it exists
        if policyName not in self.policies:
            return
        # remove it from the List
        self.policies.remove(policyName)

        # update the model
        createEpg(self.tenantName, self.networkName, self.groupName, self.policies)

    # delete the epg
    def delete(self):
        # delete the Epg
        deleteEpg(self.tenantName, self.networkName, self.groupName)

class network:
    def __init__(self, tenantName, networkName, pktTag, subnet, gateway, encap):
        netList = listNet()
        found = False
        for net in netList:
            if net['tenantName'] == tenantName and net['networkName'] == networkName:
                found = True

        #Create the network if not found
        if found == False:
            createNet(tenantName, networkName, pktTag, encap, subnet, gateway)

        # Save Parameters
        self.tenantName = tenantName
        self.networkName = networkName
        self.pktTag = pktTag
        self.encap = encap
        self.subnet = subnet
        self.gateway = gateway
        self.groups = {}

    def newGroup(self, groupName, policies=[]):
        epg = group(self.tenantName, self.networkName, groupName, policies)

        # store the epg
        self.groups[groupName] = epg

        return epg

    def deleteGroup(self, groupName):
        if groupName not in self.groups:
            return

        # delete the epg
        self.groups[groupName].delete()
        del self.groups[groupName]

    def delete(self):
        deleteNet(self.tenantName, self.networkName)

# Create policy
def createPolicy(tenantName, policyName):
    print "Creating policy {0}:{1}".format(tenantName, policyName)
    postUrl = 'http://localhost:9999/api/policys/' + tenantName + ':' + policyName + '/'

    jdata = json.dumps({
        "tenantName": tenantName,
        "policyName": policyName
    })

    # Post the data
    response = http.httpPost(postUrl, jdata)
    print "Create policy response is: " + response

    if response == "Error":
        errorExit("Policy create failure")

# Delete policy
def deletePolicy(tenantName, policyName):
    print "Deleting policy {0}:{1}".format(tenantName, policyName)

    # Delete Policy
    deleteUrl = 'http://localhost:9999/api/policys/' + tenantName + ':' + policyName + '/'
    response = http.httpDelete(deleteUrl)

    if response == "Error":
        errorExit("Policy create failure")

# List all policies
def listPolicy():
    # Get a list of policies
    retDate = urllib2.urlopen('http://localhost:9999/api/policys/')
    if retData == "Error":
        errorExit("list policy failed")

    return json.loads(retData)


# Add rule to a policy
def addRule(tenantName, policyName, ruleId, priority=1, direction='in',
    endpointGroup="", network="", ipAddress="", protocol="", port=0, action="allow"):
    print "Adding rule {2} to policy {0}:{1}".format(tenantName, policyName, ruleId)

    if direction == 'in':
        jdata = json.dumps({
          "tenantName": tenantName,
          "policyName": policyName,
          "ruleId": ruleId,
          "priority": priority,
          "direction": direction,
          "fromEndpointGroup": endpointGroup,
          "fromNetwork": network,
          "fromIpAddress": ipAddress,
          "protocol": protocol,
          "port": port,
          "action": action
         })
    else:
        jdata = json.dumps({
          "tenantName": tenantName,
          "policyName": policyName,
          "ruleId": ruleId,
          "priority": priority,
          "direction": direction,
          "toEndpointGroup": endpointGroup,
          "toNetwork": network,
          "toIpAddress": ipAddress,
          "protocol": protocol,
          "port": port,
          "action": action
         })

    #Post the data
    postUrl = 'http://localhost:9999/api/rules/' + tenantName + ':' + policyName + ':' + ruleId + '/'
    print "Adding rule " + jdata
    response = http.httpPost(postUrl, jdata)
    print "Rule add response is: " + response

    if response == "Error":
        errorExit("Rule add failure")

# Delete rule
def deleteRule(tenantName, policyName, ruleId):
    print "Deleting rule {0}:{1}:{2}".format(tenantName, policyName, ruleId)

    # Delete Rule
    deleteUrl = 'http://localhost:9999/api/rules/' + tenantName + ':' + policyName + ':' + ruleId + '/'
    response = http.httpDelete(deleteUrl)

    # Check for error
    if response == "Error":
        errorExit("rule delete failure")

# List all rules
def listRule():
    # Get the list of all rules
    return json.loads(urllib2.urlopen('http://localhost:9999/api/rules/').read())

# Create endpoint group
def createEpg(tenantName, networkName, groupName, policies=[]):
    print "Creating endpoint group {0}:{1}:{2}".format(tenantName, networkName, groupName)

    jdata = json.dumps({
      "tenantName": tenantName,
      "groupName": groupName,
      "networkName": networkName,
      "policies": policies,
     })

    # Create epg
    postUrl = 'http://localhost:9999/api/endpointGroups/' + tenantName + ':' + networkName + ':' + groupName + '/'
    response = http.httpPost(postUrl, jdata)
    print "Epg Create response is: " + response

    # Check for error
    if response == "Error":
        errorExit("Epg create failure")

# Delete endpoint group
def deleteEpg(tenantName, networkName, groupName):
    print "Deleting endpoint group {0}:{1}:{2}".format(tenantName, networkName, groupName)

    # Delete EPG
    deleteUrl = 'http://localhost:9999/api/endpointGroups/' + tenantName + ':' + networkName + ':' + groupName + '/'
    response = http.httpDelete(deleteUrl)

    # Check for error
    if response == "Error":
        errorExit("Epg delete failure")

# List all endpoint groups
def listEpg():
    # Get the list of endpoint groups
    return json.loads(urllib2.urlopen('http://localhost:9999/api/endpointGroups/').read())

# Create Network
def createNet(tenantName, networkName, pktTag, encap="vxlan", subnet="", gateway=""):
    print "Creating network {0}:{1}".format(tenantName, networkName)

    # Create network
    postUrl = 'http://localhost:9999/api/networks/' + tenantName + ':' + networkName + '/'
    jdata = json.dumps({
      "tenantName": tenantName,
      "networkName": networkName,
      "pktTag": pktTag,
      "isPublic": False,
      "isPrivate": True,
      "encap": encap,
      "subnet": subnet,
      "gateway": gateway,
     })
    response = http.httpPost(postUrl, jdata)
    print "Network Create response is: " + response

    # Check for error
    if response == "Error":
        errorExit("Network create failure")

# Delete Network
def deleteNet(tenantName, networkName):
    print "Deleting network {0}:{1}".format(tenantName, networkName)

    # Delete network
    deleteUrl = 'http://localhost:9999/api/networks/' + tenantName + ':' + networkName + '/'
    response = http.httpDelete(deleteUrl)

    # Check for error
    if response == "Error":
        errorExit("Network delete failure")

# List all Networks
def listNet():
    # Get the list of Networks
    return json.loads(urllib2.urlopen('http://localhost:9999/api/networks/').read())

# Create Tenant
def createTenant(tenantName):
    print "Creating tenant {0}".format(tenantName)

    # Create tenant
    postUrl = 'http://localhost:9999/api/tenants/' + tenantName + '/'
    jdata = json.dumps({
       "key": tenantName,
       "tenantName": tenantName,
       "subnetPool": "10.1.1.1/8",
       "subnetLen":  24,
       "vlans": "100-1100",
       "vxlans": "1000-1100",
     })
    response = http.httpPost(postUrl, jdata)
    print "Tenant Create response is: " + response

    # Check for error
    if response == "Error":
        errorExit("Tenant create failure")

# Delete Tenant
def deleteTenant(tenantName):
    print "Deleting tenant {0}".format(tenantName)

    # Delete tenant
    deleteUrl = 'http://localhost:9999/api/tenants/' + tenantName + '/'
    response = http.httpDelete(deleteUrl)

    # Check for error
    if response == "Error":
        errorExit("Tenant delete failure")

# List all Tenants
def listTenant():
    # Get the list of Tenants
    return json.loads(urllib2.urlopen('http://localhost:9999/api/tenants/').read())

# Configure ACI Mode
def setFabricMode(mode):
    postUrl = 'http://netmaster:9999/api/globals/global/'
    jdata = json.dumps({
      "name": "global",
      "network-infra-type": mode,
      "vlans": "1-4094",
      "vxlans": "1-10000",
     })
    response = http.httpPost(postUrl, jdata)

    # Check for error
    if response == "Error":
        print response
        errorExit("setFabricMode failed")
