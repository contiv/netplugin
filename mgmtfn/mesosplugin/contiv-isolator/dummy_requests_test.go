/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

const alloc = `
{
    "command": "allocate",
    "args": {
        "hostname": "slave-0-1",
        "num_ipv4": 1,
        "num_ipv6": 2,
        "uid": "0cd47986-24ad-4c00-b9d3-5db9e5c02028",
        "netgroups": ["prod", "frontend"],
        "labels": [{
              "key": "rack",
              "value": "3A"
            },
            {
              "key": "pop",
              "value": "houston"
            }
        ]
    }
}`

const release = `
{
    "command": "release",
    "args": {
        "uid": "0cd47986-24ad-4c00-b9d3-5db9e5c02028"
    }
}`

const releaseIPs = `
{
    "command": "release",
    "args": {
        "ips": ["192.168.23.4", "2001:3ac3:f90b:1111::1"]
    }
}`

const isolate = `
{
    "command": "isolate",
    "args": {
        "hostname": "slave-H3A-1",
        "container_id": "ba11f1de-fc4d-46fd-9f15-424f4ef05a3a",
        "pid": 3789,
        "ipv4_addrs": ["192.168.23.4"],
        "ipv6_addrs": ["2001:3ac3:f90b:1111::1"],
        "netgroups": ["prod", "frontend"],
        "labels": [{
              "key": "rack",
              "value": "3A"
            },
            {
              "key": "pop",
              "value": "houston"
            }
        ]
    }
}`

const cleanup = `
{
    "command": "cleanup",
    "args": {
        "hostname": "slave-H3A-1",
        "container_id": "ba11f1de-fc4d-46fd-9f15-424f4ef05a3a"
        }
}`
