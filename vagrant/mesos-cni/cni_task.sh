#!/bin/bash
#Copyright 2016 Cisco Systems Inc. All rights reserved.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

usage() {
    echo "usage: $0 [-m marathon-ipaddr] [-j jobname] [-t tenant-name] [-n network-name] [-g network-group] [-s subnet]"
    exit 0
}

die () {
    echo $1 && exit 1
}

JSON_FILE=/tmp/cni.json
MASTER_ADDR=192.168.2.10
JOB_NAME="container.$RANDOM"
SUBNET="10.36.28.0/24"
TENANT_NAME="default"
NETWORK_NAME="default-net"
NETWORK_GROUP="default"
# manual config required for network-group

while getopts "m:j:t:n:g:s:h" opt
do
  case "$opt" in

  j) 
     JOB_NAME=$OPTARG
     ;;
  t) 
     TENANT_NAME=$OPTARG   
     ;;
  n) 
     NETWORK_NAME=$OPTARG   
     ;;
  g) 
     NETWORK_GROUP=$OPTARG   
     ;;
  s) 
     SUBNET=$OPTARG   
     ;;
  h | *) 
     usage
     ;;
  esac
done

[ -z "$JOB_NAME" ] || [ -z "$SUBNET" ] || [ -z "$TENANT_NAME" ] || \
                  [ -z "$NETWORK_NAME" ] && die "invalid argument"

until netctl tenant ls | awk -F' ' '{ print $1}' | grep -w "$TENANT_NAME" > /dev/null ; do
   echo "==> netctl tenant create $TENANT_NAME"
   netctl tenant create "$TENANT_NAME"
done

if ! netctl net ls | awk -F' ' '{ print $6}' | grep -w "$SUBNET" > /dev/null; then
   until netctl net ls | awk -F' ' '{ print $2}' | grep -w "$NETWORK_NAME"  > /dev/null; do
      echo "==> netctl net create -t $TENANT_NAME -s $SUBNET $NETWORK_NAME"
      netctl net create -t "$TENANT_NAME" -s "$SUBNET" "$NETWORK_NAME"
   done
fi

rm -f "$JSON_FILE"
cat > "$JSON_FILE" << JSON_SCRIPT
{
  "id": "$JOB_NAME",
  "cmd": "python -m  SimpleHTTPServer 9002",
  "cpus": 1,
  "mem": 500,
  "disk": 0,
  "instances": 1,
  "container": {
    "type": "MESOS",
    "volumes": [],
    "mesos": {
      "image": "ubuntu:14.04",
      "privileged": false,
      "parameters": [],
      "forcePullImage": false
    }
  },
  "ipAddress": {
     "networkName": "netcontiv",
     "labels": {
     }
  }
}
JSON_SCRIPT

TMP_FILE="/tmp/jq.$$"
[ "$TENANT_NAME" != "default" ] &&
 jq --arg inarg "$TENANT_NAME" '.ipAddress.labels."io.contiv.tenant"=$inarg' "$JSON_FILE" > "$TMP_FILE" && mv "$TMP_FILE" "$JSON_FILE"
[ "$NETWORK_NAME" != "default-net" ] &&
 jq --arg inarg "$NETWORK_NAME" '.ipAddress.labels."io.contiv.network"=$inarg' "$JSON_FILE" > "$TMP_FILE" && mv "$TMP_FILE" "$JSON_FILE"
[ "$NETWORK_GROUP" != "default" ] &&
 jq --arg inarg "$NETWORK_GROUP" '.ipAddress.labels."io.contiv.net-group"=$inarg' "$JSON_FILE" > "$TMP_FILE" && mv "$TMP_FILE" "$JSON_FILE"

# create task
curl -X POST http://"$MASTER_ADDR":8080/v2/apps -d @${JSON_FILE} \
     -H "Content-type: application/json"
