import urllib
import urllib2
import json

tenantName = 'default'
policyName = 'policy1'

# Create policy
postUrl = 'http://localhost:9999/api/policys/' + tenantName + ':' + policyName + '/'
jdata = json.dumps({
  "tenantName": tenantName,
  "policyName": policyName
 })
response = urllib2.urlopen(postUrl, jdata).read()
print "Create policy response is: " + response

# Create Rules
for ruleId in ['1', '2']:
	postUrl = 'http://localhost:9999/api/rules/' + tenantName + ':' + policyName + ':' + ruleId + '/'
	jdata = json.dumps({
	  "tenantName": tenantName,
	  "policyName": policyName,
	  "ruleName": ruleId,
	  "direction": 'in' if ruleId == '1' else 'out',
	  "protocol": 'tcp' if ruleId == '1' else 'udp',
	  "port": 80,
	  "action": 'accept' if ruleId == '1' else 'deny'
	 })
	print "rule create, sending: " + jdata
	response = urllib2.urlopen(postUrl, jdata).read()
	print "Rule add POST response is: " + response

# Create endpoint groups
for epgName in ['foo', 'bar', 'baz']:
	networkName = "private"
	postUrl = 'http://localhost:9999/api/endpointGroups/' + tenantName + ':' + epgName + '/'
	jdata = json.dumps({
	  "tenantName": tenantName,
	  "groupName": epgName,
	  "networkName": networkName,
	  "policies": [policyName]
	 })
	response = urllib2.urlopen(postUrl, jdata).read()
	print "Epg Create response is: " + response

# List
print "Got Networks: "
print json.dumps(json.loads(urllib2.urlopen('http://localhost:9999/api/networks/').read()), indent=4, sort_keys=True)
print "Got EndpointGroups: "
print json.dumps(json.loads(urllib2.urlopen('http://localhost:9999/api/endpointGroups/').read()), indent=4, sort_keys=True)
print "Got Policies: "
print json.dumps(json.loads(urllib2.urlopen('http://localhost:9999/api/policys/').read()), indent=4, sort_keys=True)
print "Got Rules: "
print json.dumps(json.loads(urllib2.urlopen('http://localhost:9999/api/rules/').read()), indent=4, sort_keys=True)

# Server: netcat -k -l -p 5000
# client: netcat -z -n -v 192.168.2.10 5000
