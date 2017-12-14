
# contivModel REST client

import urllib
import urllib2
import json
import argparse
import os

# Exit on error
def errorExit(str):
    print "############### Operation failed: " + str + " ###############"
    os._exit(1)

# HTTP Delete wrapper
def httpDelete(url):
    opener = urllib2.build_opener(urllib2.HTTPHandler)
    request = urllib2.Request(url)
    request.get_method = lambda: 'DELETE'
    try:
        ret = opener.open(request)
        return ret

    except urllib2.HTTPError, err:
        if err.code == 404:
            print "Page not found!"
        elif err.code == 403:
            print "Access denied!"
        else:
            print "HTTP Error response! Error code", err.code
        return "Error"
    except urllib2.URLError, err:
        print "URL error:", err.reason
        return "Error"

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
        return "Error"
    except urllib2.URLError, err:
        print "URL error:", err.reason
        return "Error"

# object model client
class objmodelClient:
	def __init__(self, baseUrl):
		self.baseUrl = baseUrl

	# Create aciGw
	def createAciGw(self, obj):
	    postUrl = self.baseUrl + '/api/v1/aciGws/' + obj.name  + '/'

	    jdata = json.dumps({ 
			"enforcePolicies": obj.enforcePolicies, 
			"includeCommonTenant": obj.includeCommonTenant, 
			"name": obj.name, 
			"nodeBindings": obj.nodeBindings, 
			"pathBindings": obj.pathBindings, 
			"physicalDomain": obj.physicalDomain, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("AciGw create failure")

	# Delete aciGw
	def deleteAciGw(self, name):
	    # Delete AciGw
	    deleteUrl = self.baseUrl + '/api/v1/aciGws/' + name  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("AciGw create failure")

	# List all aciGw objects
	def listAciGw(self):
	    # Get a list of aciGw objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/aciGws/')
	    if retData == "Error":
	        errorExit("list AciGw failed")

	    return json.loads(retData)



	# Inspect aciGw
	def createAciGw(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/aciGw/' + obj.name  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list AciGw failed")

	    return json.loads(retData)


	# Create appProfile
	def createAppProfile(self, obj):
	    postUrl = self.baseUrl + '/api/v1/appProfiles/' + obj.tenantName + ":" + obj.appProfileName  + '/'

	    jdata = json.dumps({ 
			"appProfileName": obj.appProfileName, 
			"endpointGroups": obj.endpointGroups, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("AppProfile create failure")

	# Delete appProfile
	def deleteAppProfile(self, tenantName, appProfileName):
	    # Delete AppProfile
	    deleteUrl = self.baseUrl + '/api/v1/appProfiles/' + tenantName + ":" + appProfileName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("AppProfile create failure")

	# List all appProfile objects
	def listAppProfile(self):
	    # Get a list of appProfile objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/appProfiles/')
	    if retData == "Error":
	        errorExit("list AppProfile failed")

	    return json.loads(retData)




	# Create Bgp
	def createBgp(self, obj):
	    postUrl = self.baseUrl + '/api/v1/Bgps/' + obj.hostname  + '/'

	    jdata = json.dumps({ 
			"as": obj.as, 
			"hostname": obj.hostname, 
			"neighbor": obj.neighbor, 
			"neighbor-as": obj.neighbor-as, 
			"routerip": obj.routerip, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Bgp create failure")

	# Delete Bgp
	def deleteBgp(self, hostname):
	    # Delete Bgp
	    deleteUrl = self.baseUrl + '/api/v1/Bgps/' + hostname  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Bgp create failure")

	# List all Bgp objects
	def listBgp(self):
	    # Get a list of Bgp objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/Bgps/')
	    if retData == "Error":
	        errorExit("list Bgp failed")

	    return json.loads(retData)



	# Inspect Bgp
	def createBgp(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/Bgp/' + obj.hostname  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list Bgp failed")

	    return json.loads(retData)




	# Inspect endpoint
	def createEndpoint(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/endpoint/' + obj.endpointID  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list Endpoint failed")

	    return json.loads(retData)


	# Create endpointGroup
	def createEndpointGroup(self, obj):
	    postUrl = self.baseUrl + '/api/v1/endpointGroups/' + obj.tenantName + ":" + obj.groupName  + '/'

	    jdata = json.dumps({ 
			"cfgdTag": obj.cfgdTag, 
			"extContractsGrps": obj.extContractsGrps, 
			"groupName": obj.groupName, 
			"ipPool": obj.ipPool, 
			"netProfile": obj.netProfile, 
			"networkName": obj.networkName, 
			"policies": obj.policies, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("EndpointGroup create failure")

	# Delete endpointGroup
	def deleteEndpointGroup(self, tenantName, groupName):
	    # Delete EndpointGroup
	    deleteUrl = self.baseUrl + '/api/v1/endpointGroups/' + tenantName + ":" + groupName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("EndpointGroup create failure")

	# List all endpointGroup objects
	def listEndpointGroup(self):
	    # Get a list of endpointGroup objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/endpointGroups/')
	    if retData == "Error":
	        errorExit("list EndpointGroup failed")

	    return json.loads(retData)



	# Inspect endpointGroup
	def createEndpointGroup(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/endpointGroup/' + obj.tenantName + ":" + obj.groupName  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list EndpointGroup failed")

	    return json.loads(retData)


	# Create extContractsGroup
	def createExtContractsGroup(self, obj):
	    postUrl = self.baseUrl + '/api/v1/extContractsGroups/' + obj.tenantName + ":" + obj.contractsGroupName  + '/'

	    jdata = json.dumps({ 
			"contracts": obj.contracts, 
			"contractsGroupName": obj.contractsGroupName, 
			"contractsType": obj.contractsType, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("ExtContractsGroup create failure")

	# Delete extContractsGroup
	def deleteExtContractsGroup(self, tenantName, contractsGroupName):
	    # Delete ExtContractsGroup
	    deleteUrl = self.baseUrl + '/api/v1/extContractsGroups/' + tenantName + ":" + contractsGroupName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("ExtContractsGroup create failure")

	# List all extContractsGroup objects
	def listExtContractsGroup(self):
	    # Get a list of extContractsGroup objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/extContractsGroups/')
	    if retData == "Error":
	        errorExit("list ExtContractsGroup failed")

	    return json.loads(retData)




	# Create global
	def createGlobal(self, obj):
	    postUrl = self.baseUrl + '/api/v1/globals/' + obj.name  + '/'

	    jdata = json.dumps({ 
			"arpMode": obj.arpMode, 
			"fwdMode": obj.fwdMode, 
			"name": obj.name, 
			"networkInfraType": obj.networkInfraType, 
			"pvtSubnet": obj.pvtSubnet, 
			"vlans": obj.vlans, 
			"vxlans": obj.vxlans, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Global create failure")

	# Delete global
	def deleteGlobal(self, name):
	    # Delete Global
	    deleteUrl = self.baseUrl + '/api/v1/globals/' + name  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Global create failure")

	# List all global objects
	def listGlobal(self):
	    # Get a list of global objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/globals/')
	    if retData == "Error":
	        errorExit("list Global failed")

	    return json.loads(retData)



	# Inspect global
	def createGlobal(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/global/' + obj.name  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list Global failed")

	    return json.loads(retData)


	# Create netprofile
	def createNetprofile(self, obj):
	    postUrl = self.baseUrl + '/api/v1/netprofiles/' + obj.tenantName + ":" + obj.profileName  + '/'

	    jdata = json.dumps({ 
			"DSCP": obj.DSCP, 
			"bandwidth": obj.bandwidth, 
			"burst": obj.burst, 
			"profileName": obj.profileName, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Netprofile create failure")

	# Delete netprofile
	def deleteNetprofile(self, tenantName, profileName):
	    # Delete Netprofile
	    deleteUrl = self.baseUrl + '/api/v1/netprofiles/' + tenantName + ":" + profileName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Netprofile create failure")

	# List all netprofile objects
	def listNetprofile(self):
	    # Get a list of netprofile objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/netprofiles/')
	    if retData == "Error":
	        errorExit("list Netprofile failed")

	    return json.loads(retData)




	# Create network
	def createNetwork(self, obj):
	    postUrl = self.baseUrl + '/api/v1/networks/' + obj.tenantName + ":" + obj.networkName  + '/'

	    jdata = json.dumps({ 
			"cfgdTag": obj.cfgdTag, 
			"encap": obj.encap, 
			"gateway": obj.gateway, 
			"ipv6Gateway": obj.ipv6Gateway, 
			"ipv6Subnet": obj.ipv6Subnet, 
			"networkName": obj.networkName, 
			"nwType": obj.nwType, 
			"pktTag": obj.pktTag, 
			"subnet": obj.subnet, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Network create failure")

	# Delete network
	def deleteNetwork(self, tenantName, networkName):
	    # Delete Network
	    deleteUrl = self.baseUrl + '/api/v1/networks/' + tenantName + ":" + networkName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Network create failure")

	# List all network objects
	def listNetwork(self):
	    # Get a list of network objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/networks/')
	    if retData == "Error":
	        errorExit("list Network failed")

	    return json.loads(retData)



	# Inspect network
	def createNetwork(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/network/' + obj.tenantName + ":" + obj.networkName  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list Network failed")

	    return json.loads(retData)


	# Create policy
	def createPolicy(self, obj):
	    postUrl = self.baseUrl + '/api/v1/policys/' + obj.tenantName + ":" + obj.policyName  + '/'

	    jdata = json.dumps({ 
			"policyName": obj.policyName, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Policy create failure")

	# Delete policy
	def deletePolicy(self, tenantName, policyName):
	    # Delete Policy
	    deleteUrl = self.baseUrl + '/api/v1/policys/' + tenantName + ":" + policyName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Policy create failure")

	# List all policy objects
	def listPolicy(self):
	    # Get a list of policy objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/policys/')
	    if retData == "Error":
	        errorExit("list Policy failed")

	    return json.loads(retData)



	# Inspect policy
	def createPolicy(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/policy/' + obj.tenantName + ":" + obj.policyName  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list Policy failed")

	    return json.loads(retData)


	# Create rule
	def createRule(self, obj):
	    postUrl = self.baseUrl + '/api/v1/rules/' + obj.tenantName + ":" + obj.policyName + ":" + obj.ruleId  + '/'

	    jdata = json.dumps({ 
			"action": obj.action, 
			"direction": obj.direction, 
			"fromEndpointGroup": obj.fromEndpointGroup, 
			"fromIpAddress": obj.fromIpAddress, 
			"fromNetwork": obj.fromNetwork, 
			"policyName": obj.policyName, 
			"port": obj.port, 
			"priority": obj.priority, 
			"protocol": obj.protocol, 
			"ruleId": obj.ruleId, 
			"tenantName": obj.tenantName, 
			"toEndpointGroup": obj.toEndpointGroup, 
			"toIpAddress": obj.toIpAddress, 
			"toNetwork": obj.toNetwork, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Rule create failure")

	# Delete rule
	def deleteRule(self, tenantName, policyName, ruleId):
	    # Delete Rule
	    deleteUrl = self.baseUrl + '/api/v1/rules/' + tenantName + ":" + policyName + ":" + ruleId  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Rule create failure")

	# List all rule objects
	def listRule(self):
	    # Get a list of rule objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/rules/')
	    if retData == "Error":
	        errorExit("list Rule failed")

	    return json.loads(retData)




	# Create serviceLB
	def createServiceLB(self, obj):
	    postUrl = self.baseUrl + '/api/v1/serviceLBs/' + obj.tenantName + ":" + obj.serviceName  + '/'

	    jdata = json.dumps({ 
			"ipAddress": obj.ipAddress, 
			"networkName": obj.networkName, 
			"ports": obj.ports, 
			"selectors": obj.selectors, 
			"serviceName": obj.serviceName, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("ServiceLB create failure")

	# Delete serviceLB
	def deleteServiceLB(self, tenantName, serviceName):
	    # Delete ServiceLB
	    deleteUrl = self.baseUrl + '/api/v1/serviceLBs/' + tenantName + ":" + serviceName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("ServiceLB create failure")

	# List all serviceLB objects
	def listServiceLB(self):
	    # Get a list of serviceLB objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/serviceLBs/')
	    if retData == "Error":
	        errorExit("list ServiceLB failed")

	    return json.loads(retData)



	# Inspect serviceLB
	def createServiceLB(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/serviceLB/' + obj.tenantName + ":" + obj.serviceName  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list ServiceLB failed")

	    return json.loads(retData)


	# Create tenant
	def createTenant(self, obj):
	    postUrl = self.baseUrl + '/api/v1/tenants/' + obj.tenantName  + '/'

	    jdata = json.dumps({ 
			"defaultNetwork": obj.defaultNetwork, 
			"tenantName": obj.tenantName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Tenant create failure")

	# Delete tenant
	def deleteTenant(self, tenantName):
	    # Delete Tenant
	    deleteUrl = self.baseUrl + '/api/v1/tenants/' + tenantName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Tenant create failure")

	# List all tenant objects
	def listTenant(self):
	    # Get a list of tenant objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/tenants/')
	    if retData == "Error":
	        errorExit("list Tenant failed")

	    return json.loads(retData)



	# Inspect tenant
	def createTenant(self, obj):
	    postUrl = self.baseUrl + '/api/v1/inspect/tenant/' + obj.tenantName  + '/'

	    retDate = urllib2.urlopen(postUrl)
	    if retData == "Error":
	        errorExit("list Tenant failed")

	    return json.loads(retData)


	# Create volume
	def createVolume(self, obj):
	    postUrl = self.baseUrl + '/api/v1/volumes/' + obj.tenantName + ":" + obj.volumeName  + '/'

	    jdata = json.dumps({ 
			"datastoreType": obj.datastoreType, 
			"mountPoint": obj.mountPoint, 
			"poolName": obj.poolName, 
			"size": obj.size, 
			"tenantName": obj.tenantName, 
			"volumeName": obj.volumeName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("Volume create failure")

	# Delete volume
	def deleteVolume(self, tenantName, volumeName):
	    # Delete Volume
	    deleteUrl = self.baseUrl + '/api/v1/volumes/' + tenantName + ":" + volumeName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("Volume create failure")

	# List all volume objects
	def listVolume(self):
	    # Get a list of volume objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/volumes/')
	    if retData == "Error":
	        errorExit("list Volume failed")

	    return json.loads(retData)




	# Create volumeProfile
	def createVolumeProfile(self, obj):
	    postUrl = self.baseUrl + '/api/v1/volumeProfiles/' + obj.tenantName + ":" + obj.volumeProfileName  + '/'

	    jdata = json.dumps({ 
			"datastoreType": obj.datastoreType, 
			"mountPoint": obj.mountPoint, 
			"poolName": obj.poolName, 
			"size": obj.size, 
			"tenantName": obj.tenantName, 
			"volumeProfileName": obj.volumeProfileName, 
	    })

	    # Post the data
	    response = httpPost(postUrl, jdata)

	    if response == "Error":
	        errorExit("VolumeProfile create failure")

	# Delete volumeProfile
	def deleteVolumeProfile(self, tenantName, volumeProfileName):
	    # Delete VolumeProfile
	    deleteUrl = self.baseUrl + '/api/v1/volumeProfiles/' + tenantName + ":" + volumeProfileName  + '/'
	    response = httpDelete(deleteUrl)

	    if response == "Error":
	        errorExit("VolumeProfile create failure")

	# List all volumeProfile objects
	def listVolumeProfile(self):
	    # Get a list of volumeProfile objects
	    retDate = urllib2.urlopen(self.baseUrl + '/api/v1/volumeProfiles/')
	    if retData == "Error":
	        errorExit("list VolumeProfile failed")

	    return json.loads(retData)


