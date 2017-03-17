#!/usr/bin/env python

import urllib
import urllib2
import json
import os
import http

#Etcd client library
class etcdClient:
    def __init__(self, clientUrl):
        self.clientUrl = clientUrl

    # Get etcd key
    def getKey(self, key):
        url = self.clientUrl + '/v2/keys' + key
        retData = http.httpGet(url)
        if retData == "Error":
            return retData

        resp = json.loads(retData)
        if 'node' not in resp or 'value' not in resp['node']:
            print "Invalid response from etcd"
            print resp
            return "Error"

        return json.loads(resp['node']['value'])

    # List keys recursively
    # Get etcd key
    def listKey(self, key):
        url = self.clientUrl + '/v2/keys' + key + '?recurse'
        retData = http.httpGet(url)
        if retData == "Error":
            return retData

        # Check response
        resp = json.loads(retData)
        if 'node' not in resp or 'nodes' not in resp['node']:
            print "Invalid response from etcd"
            print resp
            return "Error"

        # Parse the responses and return the list
        respList = []
        for node in resp['node']['nodes']:
            if 'value' in node:
                respList.append(json.loads(node['value']))

        return respList

if __name__ == "__main__":
    print "Get: "
    print etcdClient('http://localhost:4001').getKey('/contiv.io/service/netmaster/192.168.2.10:9999')
    print "\nList:"
    print etcdClient('http://localhost:4001').listKey('/contiv.io/service/netmaster')
