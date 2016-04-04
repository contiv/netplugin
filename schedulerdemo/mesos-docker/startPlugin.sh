#!/bin/bash
# Start netplugin on this setup

../../scripts/python/startPlugin.py -nodes 192.168.33.10,192.168.33.11

# Create a network to launch docker containers
netctl net create contiv -s 10.1.1.0/24
