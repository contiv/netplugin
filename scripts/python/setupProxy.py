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
                    "Container"         : "proxyPort",
                    "Host"         	    : "netplugin-node1"
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
            # Read the json response
            epList = json.loads(resp)

            # Look for pro
            for ep in epList:
            	if ep['id'] == "private.default-proxyPort":
        		# Config ip and bringup interface
			print "Found the proxy endpoint, bringing up the ovs interface"
            		print "sudo ifconfig " + ep['portName'] + " " + ep['ipAddress'] + " up"
            		print os.popen("sudo ifconfig " + ep['portName'] + " " + ep['ipAddress'] + " up").read()
        		return

            print "Failed to find proxy endpoint. Retrying.."
        else:
            print "HTTP error reading endpoints. Retrying.."

        # Retry after 1 sec
        time.sleep(1)

    # Failed to create endpoint
    print "ERROR: Error finding proxy endpoint. Exiting"
    os._exit(1)

if __name__ == "__main__":
    # Call it
    setupProxy()
