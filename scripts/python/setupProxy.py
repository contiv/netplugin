#!/usr/bin/python

# sanity tests
import time
import sys
import os
import urllib
import urllib2
import json

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

# Function to setup proxy
def setupProxy():
    print "Setting up proxy"
    cfg = """{
        "Tenants" : [ {
            "Name"                      : "default",
            "Networks"  : [
            {
                "Name"                  : "private",
                "Endpoints" : [ {
                    "Container"         : "myContainer1",
                    "Host"         	    : "netplugin-node1",
                    "ServiceName"		: "proxy"
                }]
            } ]
        } ]
    }

    """

    # Write the config file
    print "Writing config file.."
    cfgFile = "/tmp/proxy-endpoint.cfg"
    f = open(cfgFile,'w')
    f.write(cfg)
    f.close()

    # Add the config to Netmaster
    print 'netdcli -add-cfg ' + cfgFile
    print os.popen('netdcli -add-cfg ' + cfgFile).read()

    print "Adding ovs interface"

    # get the endpoints
    for iter in range(5):
        resp = httpGet('http://netmaster:9999/endpoints')
        if resp != "Error":
            break
        else:
            print "HTTP error reading endpoints. Retrying.."

        time.sleep(1)

    # Read the json response
    epList = json.loads(resp)

    for ep in epList:
    	if ep['id'] == "private-myContainer1":
		# Config ip and bringup interface
    		print "sudo ifconfig " + ep['portName'] + " " + ep['ipAddress'] + " up"
    		print os.popen("sudo ifconfig " + ep['portName'] + " " + ep['ipAddress'] + " up").read()
		return

    print "ERROR: Failed to find proxy endpoint"
    os._exit(1)

if __name__ == "__main__":
    # Call it
    setupProxy()
